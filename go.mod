module github.com/guliangiam/guliangiam

go 1.27.4

require (
	cloud.google.com/go/storage v1.71.0
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/alexedwards/argon2id v1.0.0
	github.com/alicebob/miniredis/v2 v2.40.0
	github.com/aws/aws-sdk-go-v2 v1.48.0
	github.com/aws/aws-sdk-go-v2/config v1.33.19
	github.com/aws/aws-sdk-go-v2/credentials v1.21.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.115.0
	github.com/aws/aws-sdk-go-v2/service/sesv2 v1.79.9
	github.com/aws/aws-sdk-go-v2/service/sns v1.48.0
	github.com/beevik/etree v1.8.1
	github.com/casbin/casbin/v3 v3.11.0
	github.com/corazawaf/coraza/v3 v3.8.1
	github.com/emmansun/gmsm v0.45.3
	github.com/getsentry/sentry-go v0.50.0
	github.com/go-acme/lego/v4 v4.35.2
	github.com/go-chi/chi/v5 v5.5.0
	github.com/go-ldap/ldap/v3 v3.4.15
	github.com/go-sql-driver/mysql v1.10.2
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.13.0
	github.com/jcmturner/gokrb5/v8 v8.4.4
	github.com/joho/godotenv v1.5.1
	github.com/lionsoul2014/ip2region/binding/golang v0.0.0-20261112022628-200f4cc7a645
	github.com/markbates/goth v1.82.0
	github.com/mojocn/base64Captcha v1.3.8
	github.com/plutov/paypal/v4 v4.17.0
	github.com/pquerna/otp v1.5.0
	github.com/prometheus/client_golang v1.25.0
	github.com/prometheus/client_model v0.6.3
	github.com/redis/go-redis/v9 v9.23.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/russellhaering/goxmldsig v1.6.1
	github.com/smartwalle/alipay/v3 v3.2.31
	github.com/stretchr/testify v1.12.1
	github.com/stripe/stripe-go/v82 v82.5.1
	github.com/testcontainers/testcontainers-go v0.45.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.45.0
	github.com/tetratelabs/wazero v1.12.0
	github.com/wechatpay-apiv3/wechatpay-go v0.2.21
	github.com/wenlng/go-captcha-assets v1.0.7
	github.com/wenlng/go-captcha/v2 v2.0.5
	github.com/xuri/excelize/v2 v2.11.1
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.47.0
	go.opentelemetry.io/otel/sdk v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	go.opentelemetry.io/proto/otlp v1.11.1
	go.uber.org/mock v0.6.0
	golang.org/x/crypto v0.59.0
	golang.org/x/time v0.16.0
	golang.org/x/tools v0.53.0
	google.golang.org/api v0.316.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
	layeh.com/radius v0.0.0-20231213012653-1006025d24f8
)

require (
	cel.dev/expr v0.26.0 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.24.4 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.3.1 // indirect
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	cloud.google.com/go/iam v1.15.1 // indirect
	cloud.google.com/go/monitoring v1.32.1 // indirect
	dario.cat/mergo v1.0.2 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20261013165250-51075c6ea6bc // indirect
	github.com/Azure/go-ntlmssp v0.1.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.63.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.63.0 // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.27 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.39.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.44.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.52.0 // indirect
	github.com/aws/smithy-go v1.28.9 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.4 // indirect
	github.com/boombuler/barcode v1.1.0 // indirect
	github.com/casbin/govaluate v1.10.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.2.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/corazawaf/libinjection-go v0.3.6 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.8.2 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.11.3 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.40.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.8 // indirect
	github.com/go-jose/go-jose/v4 v4.1.9 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/google/s2a-go v0.1.11 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.25 // indirect
	github.com/googleapis/gax-go/v2 v2.27.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.34.0 // indirect
	github.com/hashicorp/go-uuid v1.0.4 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.4 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/kaptinlin/jsonpointer v0.4.28 // indirect
	github.com/kaptinlin/jsonschema v0.9.10 // indirect
	github.com/klauspost/compress v1.20.4 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20261101121723-be84dd9509a0 // indirect
	github.com/magefile/mage v1.17.2 // indirect
	github.com/magiconair/properties v1.18.13 // indirect
	github.com/markbates/going v1.0.3 // indirect
	github.com/miekg/dns v1.1.73 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.3.3 // indirect
	github.com/moby/moby/api v1.57.0 // indirect
	github.com/moby/moby/client v0.6.3 // indirect
	github.com/moby/patternmatcher v0.6.1 // indirect
	github.com/moby/sys/sequential v0.7.0 // indirect
	github.com/moby/sys/user v0.5.0 // indirect
	github.com/moby/sys/userns v0.2.1 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/mrjones/oauth v0.0.0-20190623134757-126b35219450 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pandatix/go-abnf v0.5.5 // indirect
	github.com/petar-dambovaliev/aho-corasick v0.0.0-20250424160509-463d218d4745 // indirect
	github.com/pingcap/errors v0.11.5-0.20210425183316-da1aaba5fb63 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/power-devops/perfstat v0.0.0-20260916203055-22a1a467d9f0 // indirect
	github.com/prometheus/common v0.74.0 // indirect
	github.com/prometheus/procfs v0.23.0 // indirect
	github.com/richardlehane/mscfb v1.0.9 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/shirou/gopsutil/v4 v4.26.10 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	github.com/smartwalle/ncrypto v1.0.4 // indirect
	github.com/smartwalle/ngx v1.1.2 // indirect
	github.com/smartwalle/nsign v1.0.9 // indirect
	github.com/spiffe/go-spiffe/v2 v2.9.0 // indirect
	github.com/tidwall/gjson v1.20.4 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.3.0 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/valllabh/ocsf-schema-golang v1.0.3 // indirect
	github.com/xuri/efp v0.0.3-0.20260924042702-217b81952dec // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	github.com/yuin/gopher-lua v1.1.2 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.47.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.72.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/image v0.48.0 // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/net v0.63.0 // indirect
	golang.org/x/oauth2 v0.38.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto v0.0.0-20261116203732-fd2c7689a808 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261116203732-fd2c7689a808 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261116203732-fd2c7689a808 // indirect
	google.golang.org/grpc v1.85.0 // indirect
	rsc.io/binaryregexp v0.2.0 // indirect
)
