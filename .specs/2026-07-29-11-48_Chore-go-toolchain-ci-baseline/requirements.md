# 需求文件：建立 Go 工具鏈與 CI 品質基線

## 來源

- Draft：無
- Type：Chore
- Owner：待確認
- Status：InProgress

## 文件定位

本 spec 接續已完成的 Config 契約與檔案輸出安全邊界，先把 module 最低版本、CI 相容矩陣、測試、lint、覆蓋率及 benchmark 契約固定下來，作為後續採用 `os.Root` 的前置條件。

本輪只建立 requirements、design、tasks，不修改 Go 程式、`go.mod`、CI、Makefile 或公開文件。後續實作必須由使用者另行指示。

參考來源：

- 專案規範：`AGENTS.md`
- 現有工具設定：`go.mod`、`.github/workflows/ci.yml`、`Makefile`
- 現有公開文件：`README.md`、`DESIGN.md`
- 前置規格：`.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/`
- Go 發布紀錄：<https://go.dev/doc/devel/release>
- setup-go：<https://github.com/actions/setup-go>
- checkout：<https://github.com/actions/checkout>
- golangci-lint Action：<https://github.com/golangci/golangci-lint-action>
- golangci-lint v2 設定：<https://golangci-lint.run/docs/configuration/file/>
- GitHub Actions 安全使用：<https://docs.github.com/en/actions/reference/security/secure-use>

## 背景

目前 `go.mod`、CI 與 README 分別宣告 Go 1.21、Go 1.21 與 Go 1.19+，和 `AGENTS.md` 的 Go 1.25+ 目標不一致。CI 使用 `ubuntu-latest`、`actions/checkout@v4`、`actions/setup-go@v5`、`golangci/golangci-lint-action@v4` 與 `version: latest`，因此 runner、Action 內容及 linter 結果都可能在未修改 repository 的情況下漂移。

`Makefile` 的 `test` 沒有啟用 race detector；lint 工具不存在時會成功跳過。CI 雖產生 coverage，但沒有 repository 內可驗證的最低門檻，Codecov 上傳失敗也不會讓工作失敗。專案目前沒有 benchmark，因此尚未滿足升版所要求的關鍵路徑壓測。

截至 2026-07-29，Go 官方發布紀錄顯示 Go 1.26.5 為現行版本；Go 下載頁提供 Go 1.25.11。兩者作為本次 CI 的現行與最低支援 patch 基線。Action tag 與完整 commit SHA 必須在實作當日由官方 repository 再次解析並記錄，不可憑記憶填入。

## 問題陳述

專案缺少單一且可重現的 Go 支援政策與品質閘門，導致最低版本宣告互相矛盾、CI 供應鏈可漂移、本機與 CI 驗證不一致，且升版沒有可比較的關鍵路徑效能基線。

## 目標

1. 將 module 最低 Go 版本設為 1.25.0，且不加入 `toolchain` directive，避免 library 強迫使用者下載特定工具鏈。
2. CI 在 Go 1.25.11 與 Go 1.26.5 執行完整 Linux race 測試。
3. CI 在 Go 1.26.5 的 macOS 與 Windows 執行可攜性測試。
4. runner 使用明確 OS label，不使用 `*-latest`。
5. 所有外部 GitHub Actions 以官方 tag 對應的完整 40 字元 commit SHA 釘選，旁註版本 tag。
6. workflow 設定最小權限 `contents: read`，不得授予寫入權限或把 secret 暴露給不必要步驟。
7. golangci-lint Action 與 golangci-lint CLI 使用明確版本，新增可驗證的 v2 `.golangci.yml`。
8. `gosec`、`govet`、`errcheck`、`ineffassign`、`staticcheck`、`gocritic`、`revive` 與格式檢查納入 lint 基線。
9. `Makefile` 的 test、race、coverage、coverage-check、lint、bench、verify 行為和 CI 對齊；缺少必要工具時必須失敗並提供固定版本安裝指令。
10. 覆蓋率低於 90.0% 時，本機與 CI 都必須失敗。
11. 新增不依賴網路、外部服務與真實磁碟吞吐的 logger 關鍵路徑 benchmark。
12. 升版前後各執行至少五次 benchmark，以 `benchstat` 比較；若中位結果退化超過 10%，需說明或修正。
13. README、DESIGN 與 CI 宣告一致的 Go 1.25+ 支援政策及驗證指令。
14. 保持公開 API、日誌格式、分級路由、檔案安全行為與資源生命週期相容。

## 非目標

1. 不在本 spec 改用 `os.Root`；原子 containment 另立實作 spec。
2. 不升級 zap、multierr 或其他 runtime dependency。
3. 不建立發布、tag、部署或自動合併流程。
4. 不引入 Docker、pre-commit、code generation 或私有 runner。
5. 不設定 repository／organization 層級的 GitHub Actions policy 或 branch protection。
6. 不對 benchmark 設定容易受共享 runner 雜訊影響的 CI 效能硬閘門；CI 只確認 benchmark 可執行。
7. 不藉 lint 升級進行架構重構；若新規則揭露產品缺陷，應另立 spec。
8. 不修改既有 logger 公開函式簽章或錯誤契約。

## 已定決策

- 最低支援版本：Go 1.25.0；CI 最低 patch：Go 1.25.11。
- 現行驗證版本：Go 1.26.5。
- `go.mod` 不加入 `toolchain` directive。
- Linux 對兩個 Go 版本執行 `-race`；macOS、Windows 對現行 Go 執行一般測試。
- Linux、macOS、Windows runner 分別使用 `ubuntu-24.04`、`macos-15`、`windows-2025`。
- workflow 僅監聽 `main`；移除不存在的 `master`。
- 不使用 `stable`、`oldstable`、`latest` 或浮動 major tag 作為執行版本。
- 外部 Action 以完整 SHA 釘選；版本 tag 只放註解，方便 Dependabot 與人工審查。
- golangci-lint 採 v2 設定格式；實作當日確認並釘選 v2.12.2，並同步固定本機安裝指令。
- 覆蓋率閘門為 90.0%，高於專案預設 80%，且低於目前已驗證的 92.9%。
- 移除不具品質閘門功能的 Codecov 上傳與 `CODECOV_TOKEN` 使用；`coverage.out` 與摘要留在 job log，不依賴外部服務判定成功。
- benchmark 比較使用固定版本 `golang.org/x/perf/cmd/benchstat`，版本於實作當日解析並記錄。
- lint 新增規則若要求行為性修改，停止該項實作並更新 spec，不以寬鬆 exclusion 隱藏。

## 待確認項目

- 無。Action 與工具完整 SHA／patch 版本屬 T0 可驗證輸入，不是可自由選擇的需求；解析結果必須來自官方來源並寫入 Implementation Notes。

## 現有行為

- `go.mod`：Go 1.21。
- README：Go 1.19+。
- CI：Go 1.21、`ubuntu-latest`、Action major tag、linter `latest`。
- CI：只測單一 OS 與單一 Go 版本。
- `make test`：沒有 `-race`。
- `make lint`：工具缺少時成功跳過。
- coverage：無 90% repository 閘門。
- benchmark：不存在。
- workflow token：未明確設定最小權限。

## 新行為

- module、README、DESIGN、CI 統一宣告 Go 1.25+。
- CI 對最低與現行 Go 版本執行 race，並覆蓋三個桌面 OS 的可攜性。
- Action、runner 與 linter 版本可由 repository 差異完整追蹤。
- `make verify` 在本機提供和 CI 一致的非修改型驗證入口。
- 缺少 linter 不再被誤判為通過。
- coverage 未達 90.0% 立即失敗。
- benchmark 可重複執行、可保存升版前後結果並用 benchstat 比較。
- 後續 `os.Root` spec 可安全依賴 Go 1.25 最低版本。

## 驗收情境

### 情境：最低與現行 Go 版本均通過 race

- 測試：CI Linux matrix
- 假設：matrix 為 1.25.11、1.26.5
- 當：執行 `go test -race -count=1 ./...`
- 那麼：兩個版本均通過，且 log 顯示實際 `go version`

### 情境：跨平台相容性保持

- 測試：CI portability matrix
- 假設：Go 1.26.5，OS 為 macOS 15 與 Windows 2025
- 當：執行 `go test -count=1 ./...`
- 那麼：兩個平台均通過；平台限定的 symlink／mode 測試依既有契約明確執行或 skip

### 情境：Action 與權限可稽核

- 測試：workflow 靜態檢查
- 當：檢查所有 `uses:` 與 `permissions`
- 那麼：每個外部 Action ref 都是 40 字元 SHA、有對應 tag 註解，workflow 僅有 `contents: read`

### 情境：lint 不可被靜默跳過

- 測試：Makefile 行為與 CI lint job
- 當：工具不存在或 lint 發現問題
- 那麼：命令回傳非零；工具存在且無 issue 才成功

### 情境：覆蓋率閘門生效

- 測試：`make coverage-check`
- 當：總覆蓋率低於 90.0%
- 那麼：命令回傳非零並顯示實際值與門檻；目前程式碼則通過

### 情境：關鍵路徑 benchmark 可比較

- 測試：logger disabled core 與結構化欄位寫入 benchmark
- 當：執行固定次數的升版前後 benchmark 並交給 benchstat
- 那麼：無磁碟／網路 I/O、輸出包含 `ns/op`、`B/op`、`allocs/op`，退化超過 10% 時有明確處置

### 情境：公開行為不回歸

- 測試：現有完整測試與 race
- 當：套用工具鏈與 CI 基線變更
- 那麼：公開 API、日誌格式、路由、安全邊界與 Close 契約保持不變

## 驗收條件

1. 六個驗收情境都有自動化命令或 CI job。
2. `go test -race -count=1 ./...` 在 Go 1.25.11 與 1.26.5 通過。
3. Go 1.26.5 在 macOS 15 與 Windows 2025 通過一般測試。
4. `go vet ./...`、golangci-lint v2、格式差異檢查通過。
5. coverage 總值不低於 90.0%。
6. benchmark 在 Go 1.25.11 與 1.26.5 各執行五次，benchstat 結果已記錄。
7. 所有 workflow Action ref 為完整 SHA，沒有 `latest`、`stable` 或浮動 major ref。
8. workflow token 權限只有 `contents: read`。
9. workflow 不讀取 `CODECOV_TOKEN`，且不依賴 Codecov 成功才判定品質。
10. `go mod tidy` 後 `go.mod`、`go.sum` 無意外 dependency 變更。
11. README、DESIGN、go.mod 與 CI 的最低 Go 版本一致。
12. 未修改公開 API 或產品行為；若 lint 需要行為性修復，已中止並另立 spec。

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | Go 1.21、1.22、1.23、1.24 使用者無法再建置新版 module | README 與 PR 明確列為相容性變更；舊版使用者停留前一 release |
| 風險 | Action tag 可移動 | 只採官方 tag 解析出的完整 commit SHA |
| 風險 | runner image 更新造成差異 | 使用明確 OS label；仍接受 image 內套件安全更新 |
| 風險 | race matrix 增加 CI 時間 | 只在 Linux 對兩個 Go 版本執行 race；跨平台只測現行版 |
| 風險 | benchmark 受本機負載影響 | 固定硬體、五次樣本、benchstat；CI 只做 smoke test |
| 風險 | 90% 門檻因編譯器 instrumentation 微幅變動 | 顯示實際值；不得無理由降低門檻 |
| 假設 | GitHub-hosted runner 支援指定 OS label | 實作時以官方文件確認；若不可用先更新 spec |

## 摘要

- 關鍵決策：Go 1.25.0 最低版本，CI 驗證 1.25.11 與 1.26.5
- 可重現性：明確 runner、完整 Action SHA、固定 linter 與 benchmark 工具版本
- 品質閘門：race、跨平台、lint、90% coverage、benchmark 比較
- 安全性：最小 workflow 權限，移除非必要 Codecov token 與外部上傳
- 下一步：審閱 design.md 與 tasks.md；未經使用者指示不得開始實作
