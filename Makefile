.PHONY: build test deploy rollback logs clean help

# 变量
APP := info-filter
VERSION := $(shell git rev-parse --short HEAD)
SERVER := root@47.83.228.126
SSH_KEY := ~/Documents/mypem/first.pem
REMOTE := /opt/makestuff/$(APP)

.DEFAULT_GOAL := help

help:
	@echo "Info Filter - 可用命令"
	@echo "────────────────────────────────"
	@echo "  make build    本地构建"
	@echo "  make test     运行测试"
	@echo ""
	@echo "部署:"
	@echo "  make deploy   部署到服务器"
	@echo ""
	@echo "回滚:"
	@echo "  make rollback V=xxx  回滚到指定版本"
	@echo ""
	@echo "其他:"
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
	@CURRENT=$$(ssh -i $(SSH_KEY) $(SERVER) "readlink $(REMOTE)/current 2>/dev/null | grep -o '[^/]*$$'"); \
	if [ "$$CURRENT" = "$(APP)-$(VERSION)" ]; then \
		echo "⏭️  $(APP)@$(VERSION) 已是最新"; \
	else \
		echo "编译..." && \
		GOOS=linux GOARCH=amd64 go build -o /tmp/$(APP)-$(VERSION) ./cmd/server && \
		echo "上传..." && \
		ssh -i $(SSH_KEY) $(SERVER) "mkdir -p $(REMOTE)/releases" && \
		scp -i $(SSH_KEY) /tmp/$(APP)-$(VERSION) $(SERVER):$(REMOTE)/releases/ && \
		ssh -i $(SSH_KEY) $(SERVER) "chmod +x $(REMOTE)/releases/$(APP)-$(VERSION) && ln -sf $(REMOTE)/releases/$(APP)-$(VERSION) $(REMOTE)/current && systemctl restart $(APP)" && \
		echo "✅ $(APP)@$(VERSION)"; \
	fi

rollback:
ifndef V
	@echo "可用版本:"
	@ssh -i $(SSH_KEY) $(SERVER) "ls -t $(REMOTE)/releases/ | head -10"
	@echo ""
	@echo "用法: make rollback V=<commit>"
else
	@echo "回滚到 $(V)..."
	@ssh -i $(SSH_KEY) $(SERVER) "\
		if [ -f $(REMOTE)/releases/$(APP)-$(V) ]; then \
			ln -sf $(REMOTE)/releases/$(APP)-$(V) $(REMOTE)/current && systemctl restart $(APP) && echo '✅ $(APP)@$(V)'; \
		else \
			echo '❌ 版本不存在: $(V)'; \
		fi"
endif

logs:
	@ssh -i $(SSH_KEY) $(SERVER) "journalctl -u $(APP) -f --no-pager -n 50"

clean:
	@rm -rf bin/
	@echo "✅ 清理完成"
