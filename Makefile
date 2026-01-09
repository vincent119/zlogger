# Makefile for zlogger
# 提供常用開發指令

.PHONY: all test coverage coverage-html lint fmt build clean help

# 預設目標
all: fmt lint test

# 執行所有測試
test:
	@echo "執行測試..."
	go test -v ./...

# 測試覆蓋率
coverage:
	@echo "產生覆蓋率報告..."
	go test -coverprofile=coverage.out ./...
	@echo "\n=== 覆蓋率摘要 ==="
	go tool cover -func=coverage.out
	@echo "\n=== 總覆蓋率 ==="
	@go tool cover -func=coverage.out | grep total

# 產生 HTML 覆蓋率報告
coverage-html: coverage
	@echo "產生 HTML 報告..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "已產生 coverage.html"

# 執行 linter
lint:
	@echo "執行 linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint 未安裝，跳過 lint 檢查"; \
		echo "安裝方式: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# 格式化程式碼
fmt:
	@echo "格式化程式碼..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# 編譯專案（驗證編譯）
build:
	@echo "編譯專案..."
	go build ./...

# 清理暫存檔案
clean:
	@echo "清理暫存檔案..."
	rm -f coverage.out coverage.html
	go clean

# 顯示幫助
help:
	@echo "可用指令："
	@echo "  make test          - 執行所有測試"
	@echo "  make coverage      - 產生覆蓋率報告"
	@echo "  make coverage-html - 產生 HTML 覆蓋率報告"
	@echo "  make lint          - 執行 golangci-lint"
	@echo "  make fmt           - 格式化程式碼"
	@echo "  make build         - 編譯專案"
	@echo "  make clean         - 清理暫存檔案"
	@echo "  make all           - 執行 fmt、lint、test"
	@echo "  make help          - 顯示此幫助訊息"
