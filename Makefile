# ===== 叉车维修培训系统 — 仓库根 Makefile =====
# 后端 Go module 在 backend/；本文件从**仓库根**执行，目标内部自行 cd backend。
# 兼容 GNU Make（Windows 用 Git Bash / WSL 或原生 make 均可；CI 为 Linux）。
#
# 可变参数：
#   GO=go  SWAG=swag  NPM=npm  COMPOSE_FILE=backend/docker-compose.yml
#
# 直接运行 make 会列出所有命令。

BACKEND      := backend
FRONTEND     := frontend
COMPOSE      ?= docker compose
COMPOSE_FILE ?= $(BACKEND)/docker-compose.yml
GO           ?= go
SWAG         ?= swag
NPM          ?= npm

.DEFAULT_GOAL := help

.PHONY: help build run test vet lint tidy fmt check check-all \
        swagger gen gen-apitypes gen-aifeatures gen-deploy \
        frontend-install frontend-dev frontend-build frontend-preview \
        frontend-test frontend-test-watch frontend-coverage frontend-typecheck \
        frontend-check frontend-clean \
        dev-up dev-down dev-reset dev \
        migrate-up migrate-down migrate-version migrate-force migrate-check \
        clean

# ---------------------------------------------------------------- 帮助
# 用 $(info) 而不是 awk/echo：Windows 原生 make 没有 sh/awk，也能正常显示。
help: ## 显示可用命令
	$(info )
	$(info 用法: make <target>)
	$(info )
	$(info   build              编译后端二进制到 backend/bin/server)
	$(info   run                启动后端（默认 :8080）)
	$(info   test               go test ./... -race -cover)
	$(info   vet                go vet ./...)
	$(info   lint               golangci-lint run --timeout=5m)
	$(info   tidy               go mod tidy)
	$(info   fmt                go fmt + gofmt -s -w)
	$(info   check              vet + test（轻量提交前检查）)
	$(info   check-all          后端 check + 前端 check)
	$(info )
	$(info   swagger            重新生成 Swagger（需 swag v1.16.4）)
	$(info   gen-apitypes       Swagger -> frontend/src/api/generated)
	$(info   gen-aifeatures     生成 frontend/src/config/aiFeatures.ts)
	$(info   gen-deploy         生成 deploy/env.defaults 与 Caddyfile 生成区)
	$(info   gen                生成链（swagger -> apitypes -> aifeatures，顺序固定）)
	$(info )
	$(info   ---- 前端（frontend/）----)
	$(info   frontend-install   npm ci（按 frontend/package-lock.json）)
	$(info   frontend-dev       Vite 开发服务器（默认 :5173）)
	$(info   frontend-build     vite build + precompress)
	$(info   frontend-preview   预览构建产物)
	$(info   frontend-typecheck vue-tsc --noEmit)
	$(info   frontend-test      vitest run)
	$(info   frontend-test-watch vitest watch)
	$(info   frontend-coverage  vitest run --coverage)
	$(info   frontend-check     typecheck + test)
	$(info   frontend-clean     清理 dist / coverage / .vite)
	$(info )
	$(info   dev-up             起本地 postgres（等健康）)
	$(info   dev-down           停本地依赖)
	$(info   dev-reset          清卷重建本地依赖（数据丢失）)
	$(info   dev                dev-up + migrate-up + run)
	$(info )
	$(info   migrate-up         执行迁移)
	$(info   migrate-down       回滚最近一次迁移)
	$(info   migrate-version    查看当前迁移版本)
	$(info   migrate-force      强制设置版本：make migrate-force VERSION=N)
	$(info   migrate-check      列 / 唯一索引对账（需已连库）)
	$(info )
	$(info   clean              清理 backend/bin 与 coverage.txt)
	$(info )
	@cd .

# ---------------------------------------------------------------- 构建 / 运行
build: ## 编译后端二进制到 backend/bin/server
	cd $(BACKEND) && $(GO) build -o bin/server ./cmd/server

run: ## go run ./cmd/server（默认 :8080）
	cd $(BACKEND) && $(GO) run ./cmd/server

# ---------------------------------------------------------------- 质量
test: ## go test ./... -race -cover
	cd $(BACKEND) && $(GO) test ./... -race -cover

vet: ## go vet ./...
	cd $(BACKEND) && $(GO) vet ./...

lint: ## golangci-lint run（需本机可用的 golangci-lint）
	cd $(BACKEND) && golangci-lint run --timeout=5m

tidy: ## go mod tidy
	cd $(BACKEND) && $(GO) mod tidy

fmt: ## go fmt + gofmt -s -w
	cd $(BACKEND) && $(GO) fmt ./... && gofmt -s -w .

check: vet test ## 提交前轻量检查（vet + test；lint 由 CI 兜底）
	@echo vet + test 通过

check-all: ## 后端 check + 前端 check（顺序执行）
	$(MAKE) check
	$(MAKE) frontend-check

# ---------------------------------------------------------------- 前端
frontend-install: ## npm ci（按 frontend/package-lock.json）
	cd $(FRONTEND) && $(NPM) ci

frontend-dev: ## Vite 开发服务器（默认 :5173）
	cd $(FRONTEND) && $(NPM) run dev

frontend-build: ## vite build + precompress
	cd $(FRONTEND) && $(NPM) run build

frontend-preview: ## 预览构建产物
	cd $(FRONTEND) && $(NPM) run preview

frontend-test: ## vitest run
	cd $(FRONTEND) && $(NPM) test

frontend-test-watch: ## vitest watch（本地反复跑）
	cd $(FRONTEND) && $(NPM) exec -- vitest

frontend-coverage: ## vitest run --coverage
	cd $(FRONTEND) && $(NPM) exec -- vitest run --coverage

frontend-typecheck: ## vue-tsc --noEmit
	cd $(FRONTEND) && $(NPM) run type-check

frontend-check: ## type-check + test（顺序固定）
	$(MAKE) frontend-typecheck
	$(MAKE) frontend-test

# ---------------------------------------------------------------- 生成
swagger: ## 重新生成 Swagger（需 swag v1.16.4；注解变更后先跑它）
	cd $(BACKEND) && $(SWAG) init -g cmd/server/main.go -o docs

gen-apitypes: ## Swagger -> frontend/src/api/generated（契约类型）
	cd $(BACKEND) && $(GO) run ./cmd/gen-apitypes

gen-aifeatures: ## 生成 frontend/src/config/aiFeatures.ts
	cd $(BACKEND) && $(GO) run ./cmd/gen-aifeatures

gen-deploy: ## 生成 deploy/env.defaults 与 Caddyfile 生成区
	cd $(BACKEND) && $(GO) run ./cmd/gen-deploy

gen: ## 生成链（swagger -> apitypes -> aifeatures，顺序固定）
	$(MAKE) swagger
	$(MAKE) gen-apitypes
	$(MAKE) gen-aifeatures

# ---------------------------------------------------------------- 本地依赖
dev-up: ## 起本地 postgres（等健康）
	$(COMPOSE) -f $(COMPOSE_FILE) up -d --wait postgres

dev-down: ## 停本地依赖
	$(COMPOSE) -f $(COMPOSE_FILE) down

dev-reset: ## 清卷重建本地依赖（数据丢失）
	$(COMPOSE) -f $(COMPOSE_FILE) down -v
	$(MAKE) dev-up

dev: ## dev-up + migrate-up + run（一步到位）
	$(MAKE) dev-up
	$(MAKE) migrate-up
	$(MAKE) run

# ---------------------------------------------------------------- 数据库迁移
migrate-up: ## 执行迁移
	cd $(BACKEND) && $(GO) run ./cmd/migrate up

migrate-down: ## 回滚最近一次迁移
	cd $(BACKEND) && $(GO) run ./cmd/migrate down

migrate-version: ## 查看当前迁移版本
	cd $(BACKEND) && $(GO) run ./cmd/migrate version

migrate-force: ## 强制设置版本：make migrate-force VERSION=N
	$(if $(VERSION),,$(error 用法: make migrate-force VERSION=N))
	cd $(BACKEND) && $(GO) run ./cmd/migrate force $(VERSION)

migrate-check: ## 列 / 唯一索引对账（需已连库）
	cd $(BACKEND) && $(GO) run ./cmd/migrate check-columns

# ---------------------------------------------------------------- 清理
ifeq ($(OS),Windows_NT)
clean: ## 清理构建产物
	@cmd /c "if exist $(BACKEND)\bin rmdir /s /q $(BACKEND)\bin"
	@cmd /c "if exist $(BACKEND)\coverage.txt del /q $(BACKEND)\coverage.txt"

frontend-clean: ## 清理 dist / coverage / .vite
	@cmd /c "if exist $(FRONTEND)\dist rmdir /s /q $(FRONTEND)\dist"
	@cmd /c "if exist $(FRONTEND)\coverage rmdir /s /q $(FRONTEND)\coverage"
	@cmd /c "if exist $(FRONTEND)\.vite rmdir /s /q $(FRONTEND)\.vite"
else
clean: ## 清理构建产物
	@rm -rf $(BACKEND)/bin
	@rm -f $(BACKEND)/coverage.txt

frontend-clean: ## 清理 dist / coverage / .vite
	@rm -rf $(FRONTEND)/dist $(FRONTEND)/coverage $(FRONTEND)/.vite
endif
