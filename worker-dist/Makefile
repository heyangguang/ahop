# AHOP Worker Makefile

# 变量定义
APP_NAME = ahop-worker
VERSION = 1.0.0
GOOS = linux
GOARCH = amd64
BUILD_DIR = build
CONFIG_FILE = config.json

# 默认目标
.PHONY: all
all: clean build

# 清理构建产物
.PHONY: clean
clean:
	@echo "清理构建产物..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(APP_NAME)
	@rm -f logs/*.log

# 下载依赖
.PHONY: deps
deps:
	@echo "下载依赖..."
	@go mod download
	@go mod tidy

# 构建应用
.PHONY: build
build: deps
	@echo "构建应用..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "-X main.version=$(VERSION) -s -w" \
		-o $(BUILD_DIR)/$(APP_NAME) cmd/main.go

# 本地运行
.PHONY: run
run: build
	@echo "启动Worker..."
	@./$(BUILD_DIR)/$(APP_NAME) -config $(CONFIG_FILE)

# 带配置文件运行
.PHONY: run-with-config
run-with-config: build
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "配置文件不存在，复制示例配置..."; \
		cp config.example.json $(CONFIG_FILE); \
	fi
	@echo "使用配置文件启动Worker..."
	@./$(BUILD_DIR)/$(APP_NAME) -config $(CONFIG_FILE)

# 环境变量运行
.PHONY: run-env
run-env: build
	@echo "使用环境变量启动Worker..."
	@export WORKER_ID=worker-dev && \
	 export WORKER_CONCURRENCY=2 && \
	 export REDIS_HOST=localhost && \
	 export REDIS_PASSWORD=Admin@123 && \
	 export DB_HOST=localhost && \
	 export DB_PASSWORD=Admin@123 && \
	 ./$(BUILD_DIR)/$(APP_NAME)

# 创建配置文件
.PHONY: config
config:
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "创建配置文件..."; \
		cp config.example.json $(CONFIG_FILE); \
		echo "配置文件已创建: $(CONFIG_FILE)"; \
		echo "请编辑配置文件后再启动Worker"; \
	else \
		echo "配置文件已存在: $(CONFIG_FILE)"; \
	fi

# 创建systemd服务文件
.PHONY: systemd
systemd:
	@echo "创建systemd服务文件..."
	@mkdir -p deploy
	@echo "systemd服务文件创建功能暂不可用"

# 创建Docker镜像
.PHONY: docker
docker: build
	@echo "构建Docker镜像..."
	@echo "Docker镜像构建功能暂不可用"

# 安装到系统
.PHONY: install
install: build systemd
	@echo "安装到系统..."
	@sudo mkdir -p /opt/ahop-worker
	@sudo cp -r $(BUILD_DIR)/* /opt/ahop-worker/
	@sudo cp config.example.json /opt/ahop-worker/
	@sudo cp deploy/ahop-worker.service /etc/systemd/system/
	@sudo systemctl daemon-reload
	@echo "安装完成! 使用以下命令启动服务:"
	@echo "  sudo systemctl enable ahop-worker"
	@echo "  sudo systemctl start ahop-worker"

# 查看日志
.PHONY: logs
logs:
	@if [ -f logs/worker.log ]; then \
		tail -f logs/worker.log; \
	else \
		echo "日志文件不存在"; \
	fi

# 测试连接
.PHONY: test
test:
	@echo "测试Redis连接..."
	@redis-cli -h localhost -p 6379 -a Admin@123 ping || echo "Redis连接失败"
	@echo "测试数据库连接..."
	@PGPASSWORD=Admin@123 psql -h localhost -U postgres -d auto_healing_platform -c "SELECT 1;" || echo "数据库连接失败"

# 显示帮助
.PHONY: help
help:
	@echo "AHOP Worker Makefile"
	@echo ""
	@echo "可用命令:"
	@echo "  make build          - 构建应用"
	@echo "  make run            - 本地运行"
	@echo "  make run-with-config- 使用配置文件运行"
	@echo "  make run-env        - 使用环境变量运行"
	@echo "  make config         - 创建配置文件"
	@echo "  make systemd        - 创建systemd服务文件"
	@echo "  make docker         - 构建Docker镜像"
	@echo "  make install        - 安装到系统"
	@echo "  make test           - 测试连接"
	@echo "  make logs           - 查看日志"
	@echo "  make clean          - 清理构建产物"
	@echo "  make help           - 显示帮助"