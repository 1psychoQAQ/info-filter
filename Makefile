.PHONY: build test deploy rollback logs clean help

# 变量
APP := info-filter
COMMIT := $(shell git rev-parse --short HEAD)
TIMESTAMP := $(shell date +%m%d%H%M)
VERSION := $(COMMIT)-$(TIMESTAMP)
SERVER := root@47.83.228.126
SSH_KEY := ~/Documents/mypem/first.pem
REMOTE := /opt/makestuff/$(APP)

.DEFAULT_GOAL := help

help:
	@echo "Info Filter - 可用命令"
	@echo "────────────────────────────────"
	@echo "  make build    本地构建"
	@echo "  make test     运行测试"
	@echo "  make deploy   部署到服务器"
	@echo "  make rollback 回滚到指定版本"
	@echo "  make logs     查看服务日志"
	@echo "  make clean    清理构建产物"
	@echo "────────────────────────────────"

build:
	@echo "构建..."
	@go build -o bin/$(APP) ./cmd/server
	@echo "✅ 构建完成: bin/$(APP)"

test:
	@go test ./...

deploy:
	@echo "编译..."
	@GOOS=linux GOARCH=amd64 go build -o /tmp/$(APP)-$(VERSION) ./cmd/server
	@echo "上传..."
	@scp -i $(SSH_KEY) /tmp/$(APP)-$(VERSION) $(SERVER):$(REMOTE)/releases/
	@ssh -i $(SSH_KEY) $(SERVER) "chmod +x $(REMOTE)/releases/$(APP)-$(VERSION) && ln -sf $(REMOTE)/releases/$(APP)-$(VERSION) $(REMOTE)/current && systemctl restart $(APP)"
	@echo "✅ $(APP)@$(VERSION) deployed"

rollback:
	@echo "可用版本:"
	@ssh -i $(SSH_KEY) $(SERVER) "ls -t $(REMOTE)/releases/ | head -10"
	@read -p "输入版本: " v; \
	ssh -i $(SSH_KEY) $(SERVER) "ln -sf $(REMOTE)/releases/$$v $(REMOTE)/current && systemctl restart $(APP)"
	@echo "✅ 回滚完成"

logs:
	@ssh -i $(SSH_KEY) $(SERVER) "journalctl -u $(APP) -f --no-pager -n 50"

clean:
	@rm -rf bin/
	@echo "✅ 清理完成"
