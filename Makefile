.PHONY: dev build test clean

dev:
	@echo "启动 go-judge..."
	@docker rm -f bots-nest-go-judge 2>/dev/null; docker run -d --name bots-nest-go-judge --privileged -p 5050:5050 bots-nest-judge:latest
	@echo "启动 Weaviate..."
	@docker rm -f bots-nest-weaviate 2>/dev/null; docker run -d --name bots-nest-weaviate -p 8079:8080 \
		-e AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED=true \
		semitechnologies/weaviate:latest
	@echo "启动后端..."
	@mkdir -p .db
	@go run ./cmd/ &
	@echo "启动前端..."
	@cd web/ui && npm run dev

build:
	@echo "构建前端..."
	@cd web/ui && npm run build
	@echo "构建后端..."
	@go build -o bots-nest ./cmd/

test:
	@echo "运行测试..."
	@go test ./... -v -count=1

docker-up:
	@echo "启动所有 Docker 服务..."
	@docker compose up -d

docker-down:
	@echo "停止所有 Docker 服务..."
	@docker compose down

docker-build:
	@echo "构建 Docker 镜像..."
	docker build -t bots-nest .
	docker build -t bots-nest-judge -f Dockerfile.judge .

clean:
	@echo "清理..."
	@rm -f bots-nest
	@rm -rf web/ui/dist
	@rm -f .db/bots-nest.db
