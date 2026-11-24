package bootstrap

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/guliangiam/guliangiam/internal/infra/radius"
	sentryinfra "github.com/guliangiam/guliangiam/internal/infra/sentry"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/db"
	platformlogging "github.com/guliangiam/guliangiam/internal/platform/logging"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/joho/godotenv"
)

// dotenvFiles 按加载顺序列出本地环境文件（后加载者覆盖先加载者，godotenv 语义）。
var dotenvFiles = []string{".env.local", ".env"}

// dsnEnvKeys 是 DSN 解析链的键名，按优先级从高到低排列。
var dsnEnvKeys = []string{"DB_DSN", "POSTGRES_DSN", "DATABASE_URL"}

// envLoadResult 记录 dotenv 加载事实：加载了哪些文件，以及哪些 DSN 键是
// dotenv 新引入的（加载前进程环境里没有）。后者是「.env 静默压过编排层注入」
// 的判定依据，只记录键名，不读取值。
type envLoadResult struct {
	Files      []string
	Introduced map[string]bool
}

// initEnv 只在非生产环境加载本地 .env 文件，返回实际加载成功的文件名。
//
// 为什么生产必须跳过：godotenv.Load 会把 .env 里「进程环境尚未设置」的键补进
// 环境变量，而 DSN 的解析优先级是 DB_DSN > POSTGRES_DSN > DATABASE_URL。若部署
// 只注入了 DATABASE_URL、工作目录又残留了 .env（内含 POSTGRES_DSN），进程会静默
// 连到 .env 指向的库，且没有任何报错——这是最难排查的一类故障。生产环境的所有
// 变量必须来自编排层（k8s Secret / systemd Environment= / 容器 env），不能来自
// 代码目录里的文件。
func initEnv() []string { return loadDotenv().Files }

func loadDotenv() (result envLoadResult) {
	if config.IsProduction() {
		return result
	}
	result.Introduced = map[string]bool{}
	before := map[string]bool{}
	for _, key := range dsnEnvKeys {
		before[key] = strings.TrimSpace(os.Getenv(key)) != ""
	}
	for _, name := range dotenvFiles {
		if _, err := os.Stat(name); err != nil {
			continue
		}
		if err := godotenv.Load(name); err == nil {
			result.Files = append(result.Files, name)
		}
	}
	for _, key := range dsnEnvKeys {
		if !before[key] && strings.TrimSpace(os.Getenv(key)) != "" {
			result.Introduced[key] = true
		}
	}
	return result
}

// logEnvironmentBootstrap 在 logger 就绪后输出「配置从哪来」的事实：
// 是否加载了本地 .env、DSN 取自哪个环境变量（只打印键名，永不打印值）。
func logEnvironmentBootstrap(dotenvLoaded []string) {
	logEnvironmentBootstrapWithOrigin(dotenvLoaded, nil)
}

func logEnvironmentBootstrapWithOrigin(dotenvLoaded []string, dotenvIntroduced map[string]bool) {
	production := config.IsProduction()

	// 生产环境本应跳过 dotenv；若仍加载到了文件，说明 APP_ENV=production 是由
	// .env 自己提供的——部署边界已经被破坏，必须留下显式告警。
	if production && len(dotenvLoaded) > 0 {
		slog.Warn("APP_ENV=production came from a local dotenv file; production must inject environment from the orchestrator",
			"dotenv", dotenvLoaded)
	}

	dotenvDesc := "none"
	switch {
	case production:
		dotenvDesc = "skipped (production)"
	case len(dotenvLoaded) > 0:
		dotenvDesc = strings.Join(dotenvLoaded, ",")
	}
	slog.Info("environment bootstrap", "production", production, "dotenv", dotenvDesc)

	// 来源键名始终打印：无任何 DSN 时显式写 "unset"（随后由 PostgresConfigFromEnv
	// 报错），避免「没有这条日志」被误读成「来源未知」。只打印键名，永不打印值。
	key := db.DataSourceNameKey()
	if key == "" {
		key = "unset"
	}
	slog.Info("postgres DSN source", "env_key", key, "precedence", "DB_DSN > POSTGRES_DSN > DATABASE_URL")
	logDataSourceConflicts(key, dotenvIntroduced)
}

// logDataSourceConflicts 在同时存在多个非空 DSN 来源时告警（OPEN-API-45）。
// 只打印键名、来源类别与优先级，永不打印 DSN 值：多个来源会让高优先级键
// 静默覆盖低优先级键，而 dotenv 引入的键正是「只注入 DATABASE_URL、工作目录
// 残留 .env」时误连数据库的根因。
func logDataSourceConflicts(winner string, dotenvIntroduced map[string]bool) {
	var present []string
	for _, key := range dsnEnvKeys {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			continue
		}
		origin := "env"
		if dotenvIntroduced[key] {
			origin = "dotenv"
		}
		present = append(present, key+"("+origin+")")
	}
	if len(present) < 2 {
		return
	}
	slog.Warn("multiple postgres DSN sources are configured; the highest-precedence one wins",
		"sources", strings.Join(present, ","),
		"env_key", winner,
		"precedence", "DB_DSN > POSTGRES_DSN > DATABASE_URL")
}

// sanitizingLoggerTo builds the startup logger on an explicit writer.
//
// The sink is JSON（规范 12.1/12.10）: platform/logging injects the resource
// attributes (service.name/version/env/caller), request correlation, §12.10
// sampling and the 32KB record cap; the sanitizer remains the outermost layer
// so sensitive attributes are redacted before serialization.
//
// The sink must be a concrete writer-based handler, never slog.Default().Handler():
// slog.SetDefault also redirects the standard `log` package into the new default
// handler, and the default handler itself writes through `log`. Wrapping it made
// the first log line re-enter the sanitizer through log's non-reentrant mutex, so
// the process hung before printing anything (no listener, no error).
func sanitizingLoggerTo(w io.Writer) *slog.Logger {
	handler := platformlogging.NewJSONHandler(w, platformlogging.Options{
		ServiceVersion: config.SentryRelease(),
		Environment:    os.Getenv("APP_ENV"),
		Level:          slog.LevelInfo,
	})
	return slog.New(platformlogging.NewSanitizingHandler(handler))
}

func sanitizingLogger() *slog.Logger { return sanitizingLoggerTo(os.Stderr) }

// validateCryptoProfile 是启动期能力探测的可替换入口：生产恒为
// platformcrypto.ValidateRuntimeProfile，测试替换它以模拟 gm 缺能力。
var validateCryptoProfile = platformcrypto.ValidateRuntimeProfile

// 启动期接缝：测试用它验证监听器创建前的裁决顺序，以及字段加密器构造失败
// 会中止启动。生产值恒为真实实现。
var (
	initKMSProvider         = config.InitKMSProvider
	validateConfig          = config.ValidateOrPanic
	constructFieldEncryptor = platformcrypto.InitFieldEncryptor
)

// applyCryptoProfile 执行启动期加密能力裁决：失败只记录日志并返回错误，
// 调用方必须在创建监听器之前中止；成功才把裁决结果发布给 /healthz。
func applyCryptoProfile() error {
	report, err := validateCryptoProfile()
	if err != nil {
		slog.Error("crypto profile validation failed", "error", err)
		return err
	}
	platformcrypto.SetRuntimeReport(report)
	if len(report.Degraded) > 0 {
		slog.Warn("crypto profile degraded",
			"profile", report.Profile,
			"degraded", report.Degraded,
			"reasons", platformcrypto.HealthReport().DegradedReasons)
	} else {
		slog.Info("crypto profile resolved", "profile", report.Profile)
	}
	return nil
}

// applyFieldEncryption 在启动期主动构造并校验字段加密器。base64、解码长度、
// 算法选择与 SM4 可用性都在这里一次性裁决（错误信息只含变量名与原因，不含
// 密钥材料），失败必须发生在创建监听器之前，而不是首次字段读写才 panic。
func applyFieldEncryption() error {
	encryptor, err := constructFieldEncryptor()
	if err != nil {
		slog.Error("field encryption initialization failed", "error", err)
		return err
	}
	slog.Info("field encryption initialized",
		"algorithm", encryptor.Algorithm().String(),
		"key_version", encryptor.CurrentKeyVersion(),
		"has_previous_key", encryptor.HasPreviousKey())
	return nil
}

// applyStartupConfig 执行监听器创建前的全部 fail-fast 配置步骤。顺序不可颠倒：
//  1. 解析 KMS provider：密钥存在性校验必须走已选 provider，Vault 托管密钥
//     不应被误判为“环境变量缺失”（PasswordHistoryPepper 同理由此受益）；
//  2. 校验环境变量（存在性与格式；密钥内容规则不在这里重复）；
//  3. 裁决加密能力档位：gm 档缺能力拒绝启动；
//  4. 构造并校验字段加密器：base64/长度/算法/SM4 可用性的唯一规则集。
func applyStartupConfig() error {
	initKMSProvider(os.Getenv("KMS_PROVIDER"))
	validateConfig()
	if err := applyCryptoProfile(); err != nil {
		return err
	}
	return applyFieldEncryption()
}

func RunServer() {
	os.Exit(runServer())
}

func runServer() int {
	opts, err := ParseStartupOptions(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, StartupUsage)
			return 0
		}
		fmt.Fprintf(os.Stderr, "%v\n\n%s\n", err, StartupUsage)
		return 2
	}

	envLoad := loadDotenv()
	// 保留默认 logger 的输出格式，同时在生产入口统一脱敏敏感属性。
	slog.SetDefault(sanitizingLogger())
	logEnvironmentBootstrapWithOrigin(envLoad.Files, envLoad.Introduced)

	// CLI 开关优先于 .env/编排层注入的 AUTO_MIGRATE/AUTO_SEED，且默认关闭：
	// 裸启动只校验结构存在，不重放迁移、不导入种子。
	decision := resolveStartupGates(opts, os.LookupEnv)
	decision.apply()
	logStartupGates(decision)

	// 监听器创建前的 fail-fast 配置裁决：KMS provider → 环境变量（密钥存在性
	// 走已选 provider）→ 加密能力档位 → 字段加密器构造/密钥内容校验。任何一项
	// 失败都在 NewHTTPServer 之前返回，不创建监听器。
	if err := applyStartupConfig(); err != nil {
		return 1
	}

	mw.InitRateLimitsFromEnv()

	// Initialize Sentry SDK (no-op when SENTRY_DSN is empty)
	sentryCfg := sentryinfra.ConfigFromEnv()
	sentryinfra.Init(sentryCfg)
	defer sentryinfra.Flush(2 * time.Second)

	addr := ResolveHTTPAddress()
	server, runtime, err := NewHTTPServer(addr)
	if err != nil {
		slog.Error("create server", "error", err)
		return 1
	}
	defer runtime.Close()

	var radiusServer *radius.Server
	if v := strconv.FormatBool(config.RADIUSEnabled()); v == valueTrue || v == "1" {
		radiusAddr := config.RADIUSAddr()
		if radiusAddr == "" {
			radiusAddr = ":1812"
		}
		radiusSecret := config.RADIUSSecret()
		if radiusSecret == "" {
			slog.Error("RADIUS_ENABLED is set but RADIUS_SECRET is not configured")
			return 1
		}
		radiusServer = radius.NewServer(radiusAddr, radiusSecret)
		radiusServer.SetAuthHandler(func(ctx context.Context, username, password string) (bool, error) {
			_ = ctx
			_ = username
			_ = password
			return false, nil
		})
		// BE-B-13：环境开关自启动路径同样落库计费报文（与管理面 start 一致）。
		if runtime.Handlers.RADIUSHandler != nil {
			radiusServer.SetAcctHandler(newRadiusAccountingHandler(
				runtime.Handlers.RADIUSHandler.UOW, runtime.Services.RadiusAccountingService))
		}
		if err := radiusServer.Start(context.Background()); err != nil {
			slog.Warn("failed to start RADIUS server", "error", err)
		} else {
			defer func() {
				if err := radiusServer.Stop(); err != nil {
					slog.Warn("RADIUS server stop failed", "error", err)
				}
			}()
			slog.Info("RADIUS server listening", "addr", radiusAddr)
		}
	}

	errCh := make(chan error, 1)
	recover.SafeGo("http-server", func() {
		slog.Info("server listening", "addr", addr)
		if certFile, keyFile := config.TLSCertFile(), config.TLSKeyFile(); certFile != "" && keyFile != "" {
			errCh <- server.ListenAndServeTLS(certFile, keyFile)
		} else {
			errCh <- server.ListenAndServe()
		}
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("server shutting down on signal", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			slog.Error("server failed", "error", err)
			return 1
		}
	}
	return 0
}
