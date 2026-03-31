.PHONY: help dev-db dev-backend dev-frontend build sqlc clean

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---------- 本地开发 ----------

dev-db: ## 启动 PostgreSQL（Docker）并初始化表结构
	@docker run -d --name dash-pg \
		-e POSTGRES_USER=dash \
		-e POSTGRES_PASSWORD=dash \
		-e POSTGRES_DB=dash \
		-p 5432:5432 \
		postgres:16-alpine 2>/dev/null || docker start dash-pg
	@echo "等待 PostgreSQL 就绪..."
	@until docker exec dash-pg pg_isready -U dash > /dev/null 2>&1; do sleep 1; done
	@for f in sql/schema/*.sql; do docker exec -i dash-pg psql -U dash -d dash < "$$f"; done
	@echo "✓ PostgreSQL 已就绪 (localhost:5432, dash/dash)"

dev-backend: sqlc ## 启动 Go 后端（热重载需自行安装 air）
	@mkdir -p frontend/dist && touch frontend/dist/.gitkeep
	@echo "→ 启动后端 :8080"
	go run .

dev-frontend: ## 启动前端 Vite 开发服务器（自动代理 /api → :8080）
	cd frontend && pnpm dev

# ---------- 构建 ----------

sqlc: ## 生成 sqlc 代码
	sqlc generate

build: sqlc ## 构建前端 + 后端二进制
	cd frontend && pnpm install && pnpm run build
	CGO_ENABLED=0 go build -o bin/dash .

clean: ## 清理构建产物
	rm -rf bin/ frontend/dist/ internal/infrastructure/database/gen/

dev-db-stop: ## 停止并删除本地 PostgreSQL 容器
	docker rm -f dash-pg 2>/dev/null || true
