# zlogger 開發與品質驗證入口

GOCMD ?= go
GOLANGCI_LINT ?= golangci-lint
GOLANGCI_LINT_VERSION ?= 2.12.2
GOVULNCHECK ?= govulncheck
GOVULNCHECK_VERSION ?= 1.6.0
COVERAGE_MIN ?= 90.0
BENCH_PATTERN ?= BenchmarkLogger

.PHONY: all test test-race coverage coverage-check coverage-html lint vuln vet fmt fmt-check bench verify build clean help

# 預設執行完整品質檢查
all: verify

# 執行所有測試
test:
	@echo "執行測試..."
	$(GOCMD) test -count=1 ./...

# 使用 race detector 執行所有測試
test-race:
	@echo "執行 race 測試..."
	$(GOCMD) test -race -count=1 ./...

# 產生測試覆蓋率
coverage:
	@echo "產生覆蓋率報告..."
	$(GOCMD) test -count=1 -covermode=atomic -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -func=coverage.out

# 驗證總覆蓋率門檻
coverage-check: coverage
	@$(GOCMD) tool cover -func=coverage.out | awk -v minimum="$(COVERAGE_MIN)" '\
		/^total:/ { gsub(/%/, "", $$3); actual = $$3 } \
		END { \
			if (actual == "") { print "無法解析總覆蓋率"; exit 1 } \
			printf "總覆蓋率 %.1f%%，門檻 %.1f%%\n", actual, minimum; \
			if ((actual + 0) < (minimum + 0)) exit 1 \
		}'

# 產生 HTML 覆蓋率報告
coverage-html: coverage
	@echo "產生 HTML 報告..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "已產生 coverage.html"

# 執行固定版本 linter
lint:
	@echo "執行 golangci-lint v$(GOLANGCI_LINT_VERSION)..."
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "找不到 golangci-lint"; \
		echo "安裝方式：go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_VERSION)"; \
		exit 1; \
	}
	@$(GOLANGCI_LINT) version | grep -q "version $(GOLANGCI_LINT_VERSION) " || { \
		$(GOLANGCI_LINT) version; \
		echo "golangci-lint 版本不符，要求 v$(GOLANGCI_LINT_VERSION)"; \
		exit 1; \
	}
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run ./...

# 使用固定版本 govulncheck 與官方即時資料庫掃描可達漏洞
vuln:
	@echo "執行 govulncheck v$(GOVULNCHECK_VERSION)..."
	@command -v $(GOVULNCHECK) >/dev/null 2>&1 || { \
		echo "找不到 govulncheck"; \
		echo "安裝方式：go install golang.org/x/vuln/cmd/govulncheck@v$(GOVULNCHECK_VERSION)"; \
		exit 1; \
	}
	@$(GOVULNCHECK) -version | grep -q "Scanner: govulncheck@v$(GOVULNCHECK_VERSION)" || { \
		$(GOVULNCHECK) -version; \
		echo "govulncheck 版本不符，要求 v$(GOVULNCHECK_VERSION)"; \
		exit 1; \
	}
	$(GOVULNCHECK) -db=https://vuln.go.dev ./...

# 執行 Go 靜態分析
vet:
	@echo "執行 go vet..."
	$(GOCMD) vet ./...

# 格式化程式碼
fmt:
	@echo "格式化程式碼..."
	$(GOCMD) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# 驗證格式但不修改檔案
fmt-check:
	@echo "驗證 Go 格式..."
	@files="$$(gofmt -s -l .)"; \
	if [ -n "$$files" ]; then \
		echo "下列檔案需要執行 gofmt -s："; \
		echo "$$files"; \
		exit 1; \
	fi

# 執行 logger 關鍵路徑 benchmark smoke test
bench:
	@echo "執行 logger benchmark..."
	$(GOCMD) test -run=NONE -bench='$(BENCH_PATTERN)' -benchmem -benchtime=100x ./...

# 執行與 CI 一致的非修改型品質檢查
verify: fmt-check vet lint test-race coverage-check bench

# 編譯專案
build:
	@echo "編譯專案..."
	$(GOCMD) build ./...

# 清理暫存檔案
clean:
	@echo "清理暫存檔案..."
	rm -f coverage.out coverage.html
	$(GOCMD) clean

# 顯示幫助
help:
	@echo "可用指令："
	@echo "  make test           - 執行所有測試"
	@echo "  make test-race      - 使用 race detector 執行所有測試"
	@echo "  make coverage       - 產生覆蓋率報告"
	@echo "  make coverage-check - 驗證覆蓋率不低於門檻"
	@echo "  make coverage-html  - 產生 HTML 覆蓋率報告"
	@echo "  make lint           - 執行固定版本 golangci-lint"
	@echo "  make vuln           - 使用固定版本 govulncheck 掃描可達漏洞"
	@echo "  make vet            - 執行 go vet"
	@echo "  make fmt            - 格式化程式碼"
	@echo "  make fmt-check      - 驗證格式但不修改檔案"
	@echo "  make bench          - 執行 logger benchmark"
	@echo "  make verify         - 執行完整品質檢查"
	@echo "  make build          - 編譯專案"
	@echo "  make clean          - 清理暫存檔案"
	@echo "  make all            - 執行 make verify"
	@echo "  make help           - 顯示此幫助訊息"
