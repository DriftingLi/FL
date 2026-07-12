.PHONY: build run test lint tidy fmt dev-up dev-down dev-reset dev migrate-up migrate-down migrate-data clean

APP_BIN := bin/server
PG_DSN := postgres://forklift:forklift123@localhost:5432/forklift_training?sslmode=disable

## 构建
build:
	go build -o $(APP_BIN) ./cmd/server

## 运行
run:
	go run ./cmd/server

## 测试
test:
	go test ./... -race -cover

## 代码检查
lint:
	golangci-lint run

## 整理依赖
tidy:
	go mod tidy

## 格式化
fmt:
	go fmt ./...
	gofmt -s -w .

## ===== 本地 Docker PostgreSQL 开发环境 =====

## 启动本地 PG 容器
dev-up:
	docker compose up -d postgres redis
	@echo "等待 PostgreSQL 就绪..."
	@for /l %%i in (1,1,30) do (pg_isready -h localhost -p 5432 -U forklift >nul 2>&1 && echo PostgreSQL ready && exit /b 0) & timeout /t 1 >nul

## 停止本地 PG 容器
dev-down:
	docker compose down

## 清除卷并重建（数据将丢失）
dev-reset:
	docker compose down -v
	$(MAKE) dev-up

## 启动容器 + 迁移 + 运行服务
dev: dev-up migrate-up run

## 执行数据库迁移（up）
migrate-up:
	go run ./cmd/migrate up

## 回滚数据库迁移（down）
migrate-down:
	go run ./cmd/migrate down

## PostgreSQL -> PostgreSQL 数据迁移（从 Python 版生产库迁移至 Go 版 schema）
## 用法: make migrate-data SOURCE="postgres://user:pass@host:5432/forklift_training?sslmode=disable"
## 可选: make migrate-data SOURCE="..." DRY_RUN=true  仅校验行数不写入
migrate-data:
	go run ./cmd/migrate-data -source "$(SOURCE)" -target "$(PG_DSN)" $(if $(DRY_RUN),-dry-run)

## 清理构建产物
clean:
	rm -rf bin/ coverage.txt
