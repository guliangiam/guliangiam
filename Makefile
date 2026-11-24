
# Image URL to use all building/pushing image targets
REGISTRY ?= casbin
IMG ?= guliangiam
IMG_TAG ?=$(shell git --no-pager log -1 --format="%ad" --date=format:"%Y%m%d")-$(shell git describe --tags --always --dirty --abbrev=6)
NAMESPACE ?= guliangiam
APP ?= guliangiam
HOST ?= test.com


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: docker-build docker-push deploy

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: ut
ut: ## UT test
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

##@ Build

.PHONY: backend
backend: fmt vet ## Build backend binary.
	go build -trimpath -ldflags="-s -w" -o bin/manager ./

.PHONY: backend-vendor
backend-vendor: vendor fmt vet ## Build backend binary with vendor.
	go build -trimpath -ldflags="-s -w" -mod=vendor -o bin/manager ./

.PHONY: frontend
frontend: ## Build frontend binary.
	cd web && pnpm install && pnpm run build && cd -

.PHONY: vendor
vendor: ## Update vendor.
	go mod vendor

.PHONY: run
run: fmt vet ## Run backend in local.
	go run ./

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${REGISTRY}/${IMG}:${IMG_TAG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${REGISTRY}/${IMG}:${IMG_TAG}

deps: ## Run dependencies for local development
	docker compose up -d db

.PHONY: ensure-all
ensure-all: ## Apply all Batch 1 PostgreSQL patches using POSTGRES_DSN or DATABASE_URL.
	@test -n "$$POSTGRES_DSN$$DATABASE_URL" || (echo "POSTGRES_DSN or DATABASE_URL is required" && exit 1)
	for f in sql/batch1/postgres/*.sql; do \
		echo "Applying $$f..."; \
		psql "$${POSTGRES_DSN:-$$DATABASE_URL}" -f "$$f"; \
	done

.PHONY: ensure-session-last-seen
ensure-session-last-seen: ## Apply the Batch 1 auth.sessions.last_seen_at PostgreSQL patch.
	$(MAKE) ensure-all

.PHONY: ensure-user-mfa-runtime
ensure-user-mfa-runtime: ## Apply the Batch 1 iam.users MFA runtime PostgreSQL patch.
	$(MAKE) ensure-all

.PHONY: ensure-login-challenges
ensure-login-challenges: ## Apply the Batch 1 auth.login_challenges PostgreSQL patch using POSTGRES_DSN or DATABASE_URL.
	@test -n "$$POSTGRES_DSN$$DATABASE_URL" || (echo "POSTGRES_DSN or DATABASE_URL is required" && exit 1)
	psql "$${POSTGRES_DSN:-$$DATABASE_URL}" -f sql/batch1/postgres/003_auth_login_challenges.sql

##@ Schema

.PHONY: db-schema-apply
db-schema-apply: ## Apply YAML-generated DDL + migrations to POSTGRES_DSN (or DATABASE_URL).
	@test -n "$$POSTGRES_DSN$$DATABASE_URL" || (echo "POSTGRES_DSN or DATABASE_URL is required" && exit 1)
	@echo "→ 生成 DDL 产物 (gen_schema --all) ..."
	python3 scripts/gen_schema.py --dialect postgresql --all
	@echo "→ 按版本顺序应用结构迁移（V1 冻结基线 + V2+ 增量）..."
	@for f in $$(find db/postgresql -maxdepth 1 -name 'V*.sql' | sort -V); do \
			echo "→ 应用 $$f ..."; \
			psql "$${POSTGRES_DSN:-$$DATABASE_URL}" -v ON_ERROR_STOP=1 -f "$$f"; \
		done
	@echo "✅ Schema 已就绪"

.PHONY: check-schema-vs-db
check-schema-vs-db: ## YAML schema ↔ PostgreSQL runtime 一致性门禁 (exit 0=对齐, 1=有差异, 2=连接错).
	@test -n "$$POSTGRES_DSN$$DATABASE_URL" || (echo "POSTGRES_DSN or DATABASE_URL is required" && exit 1)
	python3 scripts/check-schema-vs-db.py --dsn "$${POSTGRES_DSN:-$$DATABASE_URL}"

.PHONY: check-rls-policy-content
check-rls-policy-content: ## RLS POLICY 内容门禁 — 拦截 USING TRUE 恒真策略 (migrations + DB 运行时).
	python3 scripts/check-rls-policy-content.py --dsn "$${POSTGRES_DSN:-$$DATABASE_URL}"

.PHONY: check-all
check-all: ## 一次性跑所有静态 + 运行时门禁 (需要可连接的 PostgreSQL).
	bash scripts/check-layers.sh
	python3 scripts/check-schema-parity.py --file db/generated/postgresql/current.up.sql
	python3 scripts/check-rls-policy-content.py
	$(MAKE) check-schema-vs-db

lint-install: ## Install golangci-lint
	@# The following installs a specific version of golangci-lint, which is appropriate for a CI server to avoid different results from build to build
	go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.40.1

lint: ## Run golangci-lint
	@echo "---lint---"
	golangci-lint run --modules-download-mode=vendor ./...

##@ Deployment

.PHONY: deploy
deploy:  ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	helm upgrade --install ${APP} manifests/guliangiam --create-namespace --set ingress.enabled=true \
	--set "ingress.hosts[0].host=${HOST},ingress.hosts[0].paths[0].path=/,ingress.hosts[0].paths[0].pathType=ImplementationSpecific" \
	--set image.tag=${IMG_TAG} --set image.repository=${REGISTRY} --set image.name=${IMG} --version ${IMG_TAG} -n ${NAMESPACE}

.PHONY: dry-run
dry-run: ## Dry run for helm install
	helm upgrade --install ${APP} manifests/guliangiam --set ingress.enabled=true \
	--set "ingress.hosts[0].host=${HOST},ingress.hosts[0].paths[0].path=/,ingress.hosts[0].paths[0].pathType=ImplementationSpecific" \
	--set image.tag=${IMG_TAG} --set image.repository=${REGISTRY} --set image.name=${IMG} --version ${IMG_TAG} -n ${NAMESPACE} --dry-run

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	helm delete ${APP} -n ${NAMESPACE}

.PHONY: slo-budget
slo-budget: ## 发布门禁：SLO 错误预算剩余低于阈值时退出 1（需 SLO_PROMETHEUS_URL）.
	@test -n "$SLO_PROMETHEUS_URL" || (echo "SLO_PROMETHEUS_URL is required" && exit 1)
	go run ./cmd/slo-budget -registry contracts/registries/slo.yaml -prometheus "$SLO_PROMETHEUS_URL" -min-remaining "${SLO_MIN_REMAINING:-10}"
