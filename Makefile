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
		cmake --build . --target llama --config Release -j$$(nproc) && \
		cmake --build . --target llama-common --config Release -j$$(nproc)

build: llama-build
	@echo "构建前端..."
	@cd web/ui && npm run build 2>/dev/null || true
	@echo "构建后端..."
	@go build -o bots-nest ./cmd/

dev: llama-build
	@echo "启动 go-judge..."
	@docker rm -f bots-nest-go-judge 2>/dev/null; docker run -d --name bots-nest-go-judge --privileged -p 5050:5050 bots-nest-judge:latest 2>/dev/null || true
	@echo "启动 Weaviate..."
	@docker rm -f bots-nest-weaviate 2>/dev/null; docker run -d --name bots-nest-weaviate -p 8079:8080 \
		-v bots-nest-weaviate-data:/var/lib/weaviate \
		-e AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED=true \
		semitechnologies/weaviate:latest 2>/dev/null || true
	@echo "启动后端..."
	@mkdir -p .db
	@go run ./cmd/ > output.log 2>&1 &
	@echo "启动前端..."
	@cd web/ui && npm run dev

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
