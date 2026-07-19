.PHONY: dev build test clean llama-build

GO_LLAMA_DIR := third_party/go-llama.cpp
LLAMA_CPP_DIR := $(GO_LLAMA_DIR)/llama.cpp
LLAMA_BUILD_DIR := $(LLAMA_CPP_DIR)/build

$(GO_LLAMA_DIR):
	@echo "克隆 go-llama.cpp..."
	@git clone --recurse-submodules https://github.com/go-skynet/go-llama.cpp.git $(GO_LLAMA_DIR)

llama-build: $(GO_LLAMA_DIR)
	@echo "编译 llama.cpp..."
	@cd $(LLAMA_CPP_DIR) && mkdir -p build && cd build && \
		cmake .. -DBUILD_SHARED_LIBS=OFF && \
		cmake --build . --config Release -j$$(nproc)

build: llama-build
	@echo "构建前端..."
	@cd web/ui && npm run build 2>/dev/null || true
	@echo "构建后端..."
	@go build -o bots-nest ./cmd/

dev: llama-build
	@echo "启动 Docker 服务..."
	@docker rm -f bots-nest-go-judge bots-nest-weaviate 2>/dev/null || true
	@docker compose up -d go-judge weaviate 2>/dev/null || true
	@echo "启动后端..."
	@mkdir -p .db data/knowledge_files data/embedding/models
	@-pkill -f "go run.*cmd/" 2>/dev/null; pkill bots-nest 2>/dev/null || true
	@go run ./cmd/ > output.log 2>&1 &
	@sleep 2
	@echo "启动前端..."
	@cd web/ui && npm run dev
	@echo "清理后端进程..."
	@-pkill -f "go run.*cmd/" 2>/dev/null; pkill bots-nest 2>/dev/null || true

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
