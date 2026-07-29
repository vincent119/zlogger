# 設計文件：建立 Go 工具鏈與 CI 品質基線

## 設計摘要

以 Go 1.25.0 作為 module 語言與標準庫最低契約，CI 分成 Linux race、跨平台 portability、lint、coverage 與 benchmark smoke 五個責任。`Makefile` 提供相同的本機入口，`.golangci.yml` 固定 v2 規則，workflow 使用明確 runner、完整 Action SHA 與唯讀 token。

本設計刻意不加入 `toolchain` directive、不升級 runtime dependency，也不在本批採用 `os.Root`。工具鏈基線完成後，再由獨立 spec 修改檔案開啟安全模型。

## 已知契約狀態

- Module：`go 1.21`。
- 文件：README 宣告 Go 1.19+；DESIGN 記錄 Go 1.21 的 TOCTOU 限制。
- CI：Go 1.21、單一 Linux runner、未釘選完整 SHA。
- Test：CI 有 race，本機 `make test` 沒有 race。
- Coverage：目前約 92.9%，但無 repository 閘門。
- Lint：CI 使用 `latest`；本機工具缺少時成功跳過；無 `.golangci.yml`。
- Benchmark：無。
- Runtime dependency：zap v1.27.0、multierr v1.10.0。
- 本機工具鏈：Go 1.26.5 darwin/arm64。

## Bounded Context

包含：

- Go module 最低版本與相容性政策
- GitHub Actions test、portability、lint、coverage、benchmark smoke
- runner、Action、linter 及 benchstat 版本固定策略
- workflow token 最小權限
- Makefile 非修改型驗證入口
- golangci-lint v2 設定
- 關鍵 logger 路徑 benchmark
- README、DESIGN 的工具鏈與驗證說明

不包含：

- `os.Root`、safe-open 或檔案安全行為變更
- logger API、encoder、SQL core、Context fields 或 goroutine 重構
- dependency upgrade、release、Docker、部署、branch protection
- repository secrets 或 Codecov 帳號設定

## 需求對應

| 需求 | 設計處理方式 | 驗證 |
|------|--------------|------|
| Go 1.25+ | `go 1.25.0`，不加 toolchain | `go mod edit -json` |
| 最低與現行相容 | Linux 1.25.11／1.26.5 matrix | race job |
| 跨平台 | macOS 15／Windows 2025，Go 1.26.5 | portability job |
| Action 可重現 | 完整 SHA + tag 註解 | workflow 靜態檢查 |
| 最小權限 | workflow `permissions: contents: read` | YAML review |
| lint 固定 | v2 config + 固定 Action／CLI 版本 | lint job、config verify |
| coverage gate | Makefile 解析 `go tool cover -func` total | 低於 90 非零 |
| benchmark | discard core 的 logger 熱路徑 | bench + benchstat |
| 本機／CI 一致 | Makefile target 由 CI 呼叫 | `make verify` |

## 版本政策

### Go directive

`go.mod` 設為：

```go
go 1.25.0
```

不加入：

```go
toolchain go1.26.5
```

原因是本專案為 library。`go` directive 應表達最低語言與標準庫契約；`toolchain` 會影響開發者工具鏈選擇，且不是消費端相容性保證。

### CI Go matrix

| 用途 | OS | Go | 命令 |
|------|----|----|------|
| 最低版本 | ubuntu-24.04 | 1.25.11 | `go test -race -count=1 ./...` |
| 現行版本 | ubuntu-24.04 | 1.26.5 | `go test -race -count=1 ./...` |
| macOS 可攜性 | macos-15 | 1.26.5 | `go test -count=1 ./...` |
| Windows 可攜性 | windows-2025 | 1.26.5 | `go test -count=1 ./...` |

每個 job 先執行 `go version` 與 `go env GOOS GOARCH CGO_ENABLED`，讓驗收證據可追蹤。

版本升級政策：新的 Go security patch 發布時，以 PR 同步更新 CI 精確 patch；最低 minor 在支援政策改變前維持 1.25。

## GitHub Actions 設計

### 觸發與併發

- `push` 與 `pull_request` 只針對 `main`。
- 加入 workflow `concurrency`，同一 ref 新執行可取消舊執行，降低重複資源。
- 不使用 `pull_request_target`，避免不可信 PR 取得較高權限或 secrets。

### 權限

```yaml
permissions:
  contents: read
```

job 不需要 `checks: write`、`pull-requests: write`、`id-token: write` 或 package 權限。

### Action 釘選

所有 `uses:` 必須符合：

```yaml
uses: actions/checkout@<40-char-sha> # v6.0.2
```

T0 以官方 repository 驗證 annotated／lightweight tag 最終 commit，記錄完整 SHA。禁止只使用 `@v6`、`@v6.0.2` 或任意短 SHA。

預定基線：

- actions/checkout：v6.0.2
- actions/setup-go：v6.4.0
- golangci/golangci-lint-action：實作日最新 v9 patch
- golangci-lint CLI：v2.12.2

移除 Codecov Action。coverage gate 由 repository 自身命令決定，避免外部上傳失敗、token 可用性或服務狀態改變 CI 語意。

### Job 分工

1. `test-race`：Go 1.25.11／1.26.5 Linux matrix，呼叫 `make test-race`。
2. `portability`：macOS／Windows matrix，呼叫 `go test -count=1 ./...`，避免 Make shell 的平台差異。
3. `lint`：Go 1.26.5，執行 config verify、`make lint`、`go vet ./...` 與格式差異檢查。
4. `coverage`：Go 1.26.5，呼叫 `make coverage-check`，輸出 total。
5. `benchmark`：Go 1.26.5，執行固定短時間 benchmark smoke，不做效能百分比 gate。

## golangci-lint v2 設計

新增 `.golangci.yml`：

```yaml
version: "2"

run:
  timeout: 5m
  tests: true
  modules-download-mode: readonly
  relative-path-mode: gomod

linters:
  default: standard
  enable:
    - errcheck
    - gocritic
    - gosec
    - govet
    - ineffassign
    - revive
    - staticcheck

formatters:
  enable:
    - gofmt
    - goimports

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

實作時先使用固定 CLI 執行 config verify，再執行全量 lint。不得先加入 exclusion。若既有程式出現 finding：

1. 格式、註解或明確無行為差異問題，可在本分支最小修正並回歸。
2. 涉及公開契約、錯誤語意、並行或 I/O 行為，停止修改，將 finding 寫入 spec 並另立修復任務。
3. false positive 只有在附規則名稱、最小程式證據與原因後，才可設精確 exclusion。

## Makefile 設計

建議 targets：

| Target | 行為 | 是否修改檔案 |
|--------|------|--------------|
| `test` | `go test -count=1 ./...` | 否 |
| `test-race` | `go test -race -count=1 ./...` | 否 |
| `coverage` | 產生 coverage.out 並顯示摘要 | 是，僅產物 |
| `coverage-check` | coverage 後驗證 90.0% | 是，僅產物 |
| `lint` | 必須找到固定 major 的 golangci-lint，執行全量檢查 | 否 |
| `fmt` | 寫入 gofmt／goimports 結果 | 是 |
| `fmt-check` | 顯示格式差異並於有差異時失敗 | 否 |
| `bench` | 執行關鍵 benchmark 並回報記憶體 | 否 |
| `verify` | fmt-check、vet、lint、test-race、coverage-check、bench smoke | 僅 coverage 產物 |

`lint` 不得在工具缺少時回傳成功。安裝提示必須使用固定版本，不可使用 `@latest`。

coverage total 由 `go tool cover -func=coverage.out` 最後一列解析；比較以數值處理，不以字串排序。門檻可透過 `COVERAGE_MIN` 覆寫供診斷，但 CI 必須明確傳入或保留 90.0，PR 不得降低。

## Benchmark 設計

新增 `benchmark_test.go` 或放在最接近 logger 熱路徑的既有 `_test.go`。至少包含：

- `BenchmarkLoggerInfoDisabled`：使用不啟用 Info 的 core，量測 level 判斷與呼叫開銷。
- `BenchmarkLoggerInfoFields`：使用 `zapcore.AddSync(io.Discard)` 與 production encoder，量測結構化欄位 encoding，不碰 stdout、檔案或網路。

共同要求：

- `b.ReportAllocs()`。
- logger 與 core 在 timer 外建立。
- 不在迴圈建立 temp directory、檔案或隨機資料。
- 固定欄位內容，避免 benchmark 自身配置主導結果。
- 不透過全域 logger，避免測試互相污染。

升版比較：

```bash
go test -run=NONE -bench='BenchmarkLogger' -benchmem -count=5 ./... > before.txt
go test -run=NONE -bench='BenchmarkLogger' -benchmem -count=5 ./... > after.txt
benchstat before.txt after.txt
```

before 應在實作 Go directive 前以最低工具鏈執行；若實際無法取得原本 Go 1.21，必須標註比較的是同一 commit 在 Go 1.25.11 與 1.26.5 的工具鏈差異，不得宣稱為程式碼前後差異。

## 覆蓋率設計

- 以 atomic mode 產生 coverage，保持和 race／並行情境相容。
- threshold 為 90.0%。
- CI log 顯示 package 明細與 total。
- `coverage.out`、`coverage.html` 保持由 `.gitignore` 排除。
- 不依賴 Codecov badge 或外部 API。

## 文件與相容性

README：

- 安裝需求改為 Go 1.25+。
- 新增 `make verify`、race、coverage 與 benchmark 指令。
- 不再宣稱 Go 1.21 的 `os.Root` 限制是當前基線；仍保留現有 Lstat TOCTOU 行為說明，直到後續 spec 真正改用 `os.Root`。

DESIGN：

- 更新工具鏈基線與 CI matrix。
- 明確區分「最低版本已足以使用 os.Root」和「目前產品碼尚未採用 os.Root」。

相容性：

- 這是 build-time breaking policy change，不是 Go API breaking change。
- Go 1.24 以下呼叫端需升級工具鏈或停留前一 module release。
- 不加入新 runtime dependency，不改輸出格式。

## 受影響檔案計畫

| 檔案 | 預期變更 | 風險 |
|------|----------|------|
| `go.mod` | go 1.25.0 | 舊工具鏈不再支援 |
| `.github/workflows/ci.yml` | matrix、SHA、permissions、jobs | CI 時間與 YAML 錯誤 |
| `.golangci.yml` | v2 lint／formatter 基線 | 揭露既有 findings |
| `Makefile` | 對齊本機與 CI 命令 | shell 可攜性 |
| `benchmark_test.go` | logger 關鍵路徑 benchmark | benchmark 自身雜訊 |
| `README.md` | Go 版本與驗證指令 | 文件與行為不同步 |
| `DESIGN.md` | 工具鏈與後續 os.Root 邊界 | 過度宣稱安全能力 |
| 本 spec 文件 | 狀態與驗證紀錄 | 無產品風險 |

## 替代方案

| 方案 | 優點 | 缺點 | 決策 |
|------|------|------|------|
| CI 使用 `stable`／`oldstable` | 自動取得安全 patch | 結果會漂移、難重現 | 不採用 |
| Action 使用 major tag | 維護簡單 | tag 可移動、供應鏈不可重現 | 不採用 |
| 加入 toolchain directive | 開發者版本一致 | library 會影響工具鏈選擇 | 不採用 |
| 保留 Codecov | 外部 dashboard | token／服務依賴且現況不阻擋失敗 | 不採用 |
| CI 強制 benchmark 10% gate | 自動阻止退化 | shared runner 雜訊高 | 不採用；人工 benchstat |
| 只測 Go 1.26 | CI 較快 | 無法證明最低版本 | 不採用 |

## 實作注意事項

- T0 必須先解析所有外部版本與 SHA，並把證據寫入 tasks Implementation Notes。
- 先取得現有 commit 的測試、coverage、lint 與 benchmark 可行性基線，再改 go.mod／CI。
- workflow YAML 必須以 GitHub 可接受的 expression 語法驗證，不以肉眼判定。
- Windows job 不呼叫依賴 POSIX shell 的 Makefile target。
- 不把 benchmark output、coverage.out 或工具 binary 加入 Git。
- 不因 Go 版本已提高就直接改用 `os.Root`；該行為需獨立 TDD spec。
- 如果固定 linter 發現行為性問題，先回報並更新需求，不在本批擴張。
