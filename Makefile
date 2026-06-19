.PHONY: dev build test clean

dev:
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

clean:
	@echo "清理..."
	@rm -f bots-nest
	@rm -rf web/ui/dist
	@rm -f .db/bots-nest.db
