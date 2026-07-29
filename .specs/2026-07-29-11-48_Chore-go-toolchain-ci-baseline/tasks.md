# 任務文件：建立 Go 工具鏈與 CI 品質基線

Status: Complete

## Execution Context

- 意圖：統一 Go 1.25+ 支援政策，建立可重現的 race、跨平台、lint、coverage 與 benchmark CI 基線
- 本輪授權：使用者已指示 `task go`，依本文件執行實作與驗證，不執行 commit、push 或發布
- 非目標：不採用 os.Root、不升級 runtime dependency、不改 logger 公開行為、不發布
- 已定決策：go 1.25.0；CI 1.25.11／1.26.5；明確 OS label；Action 完整 SHA；90% coverage；移除 Codecov；不加 toolchain directive
- 關鍵檔案：`go.mod`、`.github/workflows/ci.yml`、`.golangci.yml`、`Makefile`、`benchmark_test.go`、`README.md`、`DESIGN.md`
- 完成條件：最低／現行 race、跨平台、lint v2、90% coverage、benchmark 比較與文件一致性全部通過

### Protected Behavior

- 所有既有 exported API 與錯誤分類保持不變。
- 日誌格式、欄位、level routing、SplitOutput 換檔與 Close 生命週期保持不變。
- 檔案 leaf 驗證、0600／0700 權限與既有 symlink 防護保持不變。
- zap、multierr 與其他 runtime dependency 版本不變。
- 尚未採用 os.Root，不得宣稱已消除 TOCTOU。

### 邊界

#### Allowed Changes

實作階段限於：

- `go.mod`
- `go.sum`，只允許 `go mod tidy` 必要的格式或 checksum 變化
- `.github/workflows/ci.yml`
- `.golangci.yml`
- `Makefile`
- `benchmark_test.go` 或既有對應 `_test.go`
- `coverage.out`，只允許移除歷史上誤提交且已由 `.gitignore` 排除的測試產物
- 只為新 lint 明確要求的最小非行為性 `.go` 修正
- `README.md`
- `DESIGN.md`
- 本 spec 目錄內文件

#### Forbidden

- 新增或升級 runtime dependency
- `os.Root`、file opener、symlink、mode 或路徑驗證行為變更
- 公開 API、Config、encoder、SQL core、Context fields、全域 logger 或 SplitOutput 架構重構
- release、tag、Docker、部署、branch protection、repository secrets
- `CODECOV_TOKEN` 或其他新 secret
- 浮動 `latest`、`stable`、`oldstable`、Action major ref
- 為通過 lint 加入未具體說明的廣泛 exclusion

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T0 固定外部版本與 SHA | 無 | Complete | 官方來源驗證完成 |
| T1 記錄升版前基線 | T0 | Complete | race、coverage、vet、lint 已記錄 |
| T2 升級 Go module 契約 | T1 | Complete | 不加 toolchain，dependency 不變 |
| T3 建立 Makefile 品質入口 | T2 | Complete | 成功與失敗路徑均驗證 |
| T4 建立 golangci-lint v2 基線 | T2 | Complete | v2.12.2 為 0 issues |
| T5 新增關鍵路徑 benchmark | T2 | Complete | 無外部 I/O |
| T6 重建 GitHub Actions workflow | T3、T4、T5 | Complete | 靜態驗證完成，待遠端執行 |
| T7 更新 README 與 DESIGN | T2 至 T6 | Complete | 未宣稱已採用 os.Root |
| T8 完整驗證與邊界檢查 | T1 至 T7 | Complete | 本機與遠端跨平台 CI 全部通過 |

## 實作任務

- [x] T0 固定官方版本與完整 SHA
  - Status: Complete
  - Boundary：只更新本 tasks 的 Implementation Notes，不修改產品或 CI
  - Depends：無
  - Context：由 Go 官方發布頁確認 1.25.11／1.26.5；由各 Action 官方 repository 解析 checkout v6.0.2、setup-go v6.4.0、golangci-lint-action 最新 v9 patch 的 tag 最終 commit；固定 golangci-lint v2.12.2 與 benchstat module commit。記錄 40 字元 SHA、tag、查詢日期及來源。
  - Verify：SHA 必須為 40 個十六進位字元；tag commit 位於官方 repository；不得使用 fork 或搜尋摘要作唯一證據

- [x] T1 記錄升版前品質與效能基線
  - Status: Complete
  - Boundary：允許暫存測試輸出於未追蹤檔；只更新本 tasks，不修改產品碼
  - Depends：T0
  - Context：在目前 commit 記錄 `go version`、race、coverage、vet、現有 lint 結果。benchmark 尚不存在時，先記錄「無可比較 benchmark」，不得捏造前測數字；T5 完成後以同一程式碼分別用 Go 1.25.11、1.26.5 執行五次工具鏈比較。
  - Verify：`GOCACHE=/private/tmp/zlogger-go-cache go test -race -count=1 ./...`、coverage total、`go vet ./...`、`git status --short`

- [x] T2 升級 Go module 最低版本
  - Status: Complete
  - Boundary：`go.mod`、必要時 `go.sum`
  - Depends：T1
  - Context：將 directive 改為 `go 1.25.0`，不加入 toolchain，不升級 dependency。執行 `go mod tidy` 後檢查 module graph。
  - Verify：`go mod edit -json` 顯示 Go 1.25.0；`go list -m all`；`git diff -- go.mod go.sum` 無 dependency 版本漂移

- [x] T3 建立一致的 Makefile 品質入口
  - Status: Complete
  - Boundary：`Makefile`
  - Depends：T2
  - Context：新增 `test-race`、`coverage-check`、`fmt-check`、`bench`、`verify`；`test` 加 `-count=1`；coverage gate 90.0%；lint 缺少時非零並提供固定版本安裝指令。`fmt-check` 不修改檔案，`fmt` 保留為明確修改型命令。
  - Verify：逐一執行 target；模擬 PATH 找不到 golangci-lint 時 `make lint` 必須失敗；低門檻通過與高於實際值門檻失敗

- [x] T4 建立 golangci-lint v2 基線
  - Status: Complete
  - Boundary：`.golangci.yml`；只允許必要的最小非行為性 `.go` 修正
  - Depends：T2
  - Context：依 design 新增 v2 config，固定 standard 加 errcheck、gocritic、gosec、govet、ineffassign、revive、staticcheck，以及 gofmt、goimports formatter。先驗證 config，再全量執行。行為性 finding 停止並回報。
  - Verify：固定版本 `golangci-lint config verify` 與 `golangci-lint run ./...`；`git diff` 證明沒有廣泛 exclusion 或產品行為變更

- [x] T5 新增 logger 關鍵路徑 benchmark
  - Status: Complete
  - Boundary：`benchmark_test.go` 或最接近 logger 的既有 `_test.go`
  - Depends：T2
  - Context：新增 disabled Info 與 discard structured fields benchmark；timer 外建立 logger／core；ReportAllocs；不得使用 stdout、檔案、網路、sleep 或隨機資料。
  - Verify：`go test -run=NONE -bench='BenchmarkLogger' -benchmem -count=5 ./...`；輸出有 ns/op、B/op、allocs/op；一般測試仍通過

- [x] T6 重建 GitHub Actions CI
  - Status: Complete
  - Boundary：`.github/workflows/ci.yml`
  - Depends：T3、T4、T5
  - Context：只監聽 main；明確 concurrency 與 `contents: read`；使用 T0 的完整 Action SHA；建立 test-race、portability、lint、coverage、benchmark jobs；移除 master、ubuntu-latest、舊 Action、linter latest、Codecov 與 token。
  - Verify：YAML parse；所有 `uses:` 符合 `@[0-9a-f]{40}`；無 latest／stable／oldstable／CODECOV_TOKEN；matrix 與 design 一致

- [x] T7 更新公開文件
  - Status: Complete
  - Boundary：`README.md`、`DESIGN.md`
  - Depends：T2 至 T6
  - Context：統一 Go 1.25+，補本機 verify、CI matrix、coverage 與 benchmark 指令。更新 TOCTOU 說明為「基線已具 os.Root 能力但尚未採用」，不得宣稱原子 containment 已完成。
  - Verify：`rg -n 'Go 1\.(19|21)|Go 1.25|1.25.11|1.26.5|make verify|coverage|benchmark|os.Root|TOCTOU' README.md DESIGN.md go.mod .github/workflows/ci.yml`

## 驗證任務

- [x] T8 最低與現行版本驗證
  - Go 1.25.11：`go test -race -count=1 ./...`
  - Go 1.26.5：`go test -race -count=1 ./...`
  - 兩者均記錄 `go version` 與 GOOS／GOARCH

- [x] T9 跨平台與品質閘門
  - macOS 15／Windows 2025：Go 1.26.5 `go test -count=1 ./...`
  - `make fmt-check`
  - `go vet ./...`
  - `make lint`
  - `make coverage-check`，total >= 90.0%
  - `make bench`

- [x] T10 benchmark 相容性比較
  - 同一 commit、同一硬體下，Go 1.25.11 與 Go 1.26.5 各 `-count=5`
  - 固定 benchstat 版本比較
  - 任一 ns/op、B/op 或 allocs/op 退化超過 10% 時記錄分析；程式碼造成者需修正，工具鏈差異需在 PR 揭露

- [x] T11 安全與邊界檢查
  - workflow `permissions` 只有 `contents: read`
  - 所有外部 Action 完整 SHA 且來源為官方 repository
  - 無 Codecov token、無新 secrets、無 `pull_request_target`
  - `go mod tidy` 無 runtime dependency upgrade
  - `git diff --check` 通過
  - 差異只包含 Allowed Changes
  - 公開 API、檔案安全行為及 logger 輸出無變更
  - 不含 benchmark／coverage 產物

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 Execution Context
2. 目前未完成 task
3. Protected Behavior
4. Implementation Notes

定位命令：

```bash
rg -n '^#|^##|^###|Boundary|Depends|Status|Implementation Notes' .specs/2026-07-29-11-48_Chore-go-toolchain-ci-baseline
```

## Implementation Notes

- 2026-07-29：建立分支 `chore/go-toolchain-ci-baseline`。
- 2026-07-29：完成 requirements、design、tasks 初稿；尚未修改產品碼、測試、go.mod、CI、Makefile、README 或 DESIGN。
- 2026-07-29：現況為 go.mod／CI Go 1.21、README Go 1.19+；本機 Go 1.26.5。
- 2026-07-29：確認 CI 使用浮動 runner、Action major tag及 linter latest，Makefile lint 可靜默跳過，且無 benchmark 與 coverage gate。
- 2026-07-29：官方發布資料確認 Go 1.26.5 與 Go 1.25.11；完整 Action SHA 留待 T0 由官方 tag 解析，不在文件中猜測。
- 2026-07-29：本 spec 只建立工具鏈與 CI 前置基線，os.Root 原子 containment 另立下一份 spec。
- 2026-07-29：使用者指示 `task go`，spec 狀態改為 InProgress；本批授權實作與驗證，不含 commit、push 或發布。
- 2026-07-29：T0 從官方 Git repository 驗證 checkout v6.0.2=`de0fac2e4500dabe0009e67214ff5f5447ce83dd`、setup-go v6.4.0=`4a3601121dd01d1626a1e23e37211e3254c1c06c`、golangci-lint-action v9.3.0 tag 最終 commit=`ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a`。
- 2026-07-29：T0 初查 v2.11 系列後發現本機已為 v2.12.1，因此補查官方 tag，最終固定 golangci-lint v2.12.2=`c0d3ddc9cf3faa61a4e378e879ece580256d76e5`；benchstat 固定官方 x/perf commit=`82a0b07e230d76fa1b3036c383d7a98172f87334`。
- 2026-07-29：T1 升版前基線為 Go 1.26.5 darwin/arm64、CGO_ENABLED=1；race 與 go vet 通過；atomic coverage 92.9%；golangci-lint v2.12.1 預設設定為 0 issues，但因 sandbox cache 權限產生不影響結果的 warning。
- 2026-07-29：T1 確認 repository 沒有任何 `Benchmark*`，因此無可捏造的程式碼升版前 benchmark；T5 完成後改以同一 commit 的 Go 1.25.11／1.26.5 工具鏈比較。
- 2026-07-29：執行 T3 coverage 時發現 `coverage.out` 雖已列入 `.gitignore`，但仍被 Git 追蹤；為確保驗證不污染工作樹，將其納入 Allowed Changes 並從 repository 移除。
- 2026-07-29：T2 將 go directive 升至 1.25.0，未加入 toolchain；`go mod tidy` 未改變 zap、multierr 或其他 dependency。
- 2026-07-29：T3 新增 test-race、coverage-check、fmt-check、vet、bench、verify；coverage 92.9% 通過 90.0% 門檻，99.0% 診斷門檻會如預期失敗；linter 缺少或版本不符亦會失敗。
- 2026-07-29：T4 以 golangci-lint v2.12.2 驗證為 0 issues。新增規則揭露 7 個受控路徑／權限測試 gosec finding 與 5 個 revive 命名 finding；前者逐行附安全理由，後者只做非行為性命名修正。
- 2026-07-29：T5 新增 disabled Info 與 JSON fields benchmark；皆使用 io.Discard，無磁碟、stdout、網路或全域 logger。
- 2026-07-29：T6 workflow YAML 語法有效，所有 Action ref 為完整 40 字元 SHA，未含浮動版本、Codecov、secret 或 pull_request_target；遠端 GitHub-hosted runner 尚待 push 後執行。
- 2026-07-29：GitHub 官方 runner 文件確認 `ubuntu-24.04`、`macos-15`、`windows-2025` 均為有效標準 runner label；Go 1.26.5 的 Windows amd64 與 Linux amd64 測試 binary 交叉編譯成功。
- 2026-07-29：T7 README 與 DESIGN 已統一 Go 1.25+、CI matrix、90% coverage 與 benchmark 指令，並明確保留 Lstat TOCTOU 風險。
- 2026-07-29：本機 `make verify` 通過；Go 1.25.11 darwin/arm64 與 Go 1.26.5 darwin/arm64 的 race 均通過。
- 2026-07-29：固定 benchstat 以每組 10 次樣本比較：disabled 路徑無顯著差異；fields 路徑 Go 1.26.5 為 -29.26% sec/op；B/op 與 allocs/op 均不變。樣本執行時間變異偏高，但沒有超過 10% 的退化證據。
- 2026-07-29：T8 的 Go 1.25.11／1.26.5 race 與 T11 的安全、依賴、diff、產物及變更邊界檢查均完成；T9 保留等待 push 後的 GitHub-hosted macOS 15／Windows 2025 實際執行。
- 2026-07-29：PR #5 的 GitHub Actions run 30425988956 七項檢查全部通過；Windows 2025 job 90492470365、macOS 15、Go 1.25.11／1.26.5 race、lint、coverage 與 benchmark 均完成。T9 與本 tasks 狀態更新為 Complete。

## 後續改善

- [ ] 基線合併後另立 os.Root 原子 containment TDD spec
- [ ] 評估 govulncheck dependency 漏洞掃描與固定資料庫策略
- [ ] 評估 Dependabot 的 Go modules 與 GitHub Actions 更新排程
- [ ] 完成 Context fields defensive copy
- [ ] 清理 encoder 假契約與 SQL dead code
