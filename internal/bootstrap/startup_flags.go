package bootstrap

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// StartupOptions 记录服务启动时命令行显式给出的数据库开关。
// nil 表示「命令行没有点名」，此时回退到环境变量，再回退到默认关闭。
type StartupOptions struct {
	Migrate *bool
	Seed    *bool
}

// ParseStartupOptions 解析服务启动参数：
//
//	-migrate  启动前重放待应用的结构迁移
//	-seed     启动前导入种子数据（隐含 -migrate）
//
// 两个参数默认都不出现：裸启动不得重放数据库。环境变量
// AUTO_MIGRATE / AUTO_SEED 继续受支持，以便既有 .env 与部署清单不需改动；
// 命令行显式给出时优先级高于环境变量。
func ParseStartupOptions(args []string) (StartupOptions, error) {
	fs := flag.NewFlagSet("guliangiam", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	migrate := fs.Bool("migrate", false, "apply pending structure migrations before serving")
	seed := fs.Bool("seed", false, "apply the seed chain before serving (implies -migrate)")
	if err := fs.Parse(args); err != nil {
		return StartupOptions{}, err
	}
	if fs.NArg() > 0 {
		return StartupOptions{}, fmt.Errorf("unexpected arguments %v: only -migrate and -seed are accepted", fs.Args())
	}

	var opts StartupOptions
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "migrate":
			v := *migrate
			opts.Migrate = &v
		case "seed":
			v := *seed
			opts.Seed = &v
		}
	})
	return opts, nil
}

// StartupUsage 是启动参数的说明文本，供 -h 与错误提示使用。
const StartupUsage = `usage: guliangiam [-migrate] [-seed]

  -migrate  启动前应用待执行的结构迁移（默认取 AUTO_MIGRATE，未设置则不执行）
  -seed     启动前导入种子数据，隐含 -migrate（默认取 AUTO_SEED，未设置则不执行）

不带参数启动时只校验结构是否存在，不重放迁移、不导入种子。`

type startupGateDecision struct {
	migrate       bool
	seed          bool
	migrateSource string
	seedSource    string
}

// resolveStartupGates 按「命令行 > 环境变量 > 默认关闭」的优先级裁决两个开关。
// seed 开启时强制打开 migrate：数据链引用结构链，先导入种子会让空库直接失败。
func resolveStartupGates(opts StartupOptions, lookup func(string) (string, bool)) startupGateDecision {
	d := startupGateDecision{migrateSource: "default", seedSource: "default"}

	switch {
	case opts.Migrate != nil:
		d.migrate = *opts.Migrate
		d.migrateSource = "flag"
	default:
		if raw, ok := lookup("AUTO_MIGRATE"); ok {
			d.migrate = envTruthy(raw)
			d.migrateSource = "env"
		}
	}

	switch {
	case opts.Seed != nil:
		d.seed = *opts.Seed
		d.seedSource = "flag"
	default:
		if raw, ok := lookup("AUTO_SEED"); ok {
			d.seed = envTruthy(raw)
			d.seedSource = "env"
		}
	}

	if d.seed && !d.migrate {
		d.migrate = true
		d.migrateSource = "implied-by-seed"
	}
	return d
}

// envTruthy 与 config/db 层的判定保持同一语义（true/1/yes/on）。
func envTruthy(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case valueTrue, "1", "yes", "on":
		return true
	default:
		return false
	}
}

// apply 把裁决结果写回环境变量。db.Migrate 与 EnsureBootstrapAdminCredential
// 都通过 config.AutoMigrate/AutoSeed 读取开关；写回后 CLI 选择成为进程内唯一
// 事实，避免两层各自解析出不同结论。
func (d startupGateDecision) apply() {
	_ = os.Setenv("AUTO_MIGRATE", strconv.FormatBool(d.migrate))
	_ = os.Setenv("AUTO_SEED", strconv.FormatBool(d.seed))
}

// logStartupGates 输出「本次启动是否会重放/导入」以及每个开关的来源。
func logStartupGates(d startupGateDecision) {
	slog.Info("startup database switches resolved",
		"migrate", d.migrate, "migrate_source", d.migrateSource,
		"seed", d.seed, "seed_source", d.seedSource,
	)
	if !d.migrate && !d.seed {
		slog.Info("startup database bootstrap disabled",
			"hint", "pass -migrate to replay structure migrations and -seed to import seed data, or set AUTO_MIGRATE/AUTO_SEED=true")
	}
}
