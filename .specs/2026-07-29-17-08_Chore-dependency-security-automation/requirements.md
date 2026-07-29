# 需求文件：依賴安全自動化

## 來源

- Draft: 無
- Type: Chore
- Owner: vincent119
- Status: Complete

## 文件定位

本 spec 接續 `.specs/2026-07-29-11-48_Chore-go-toolchain-ci-baseline/tasks.md` 原先尚未完成的 `govulncheck` 與 Dependabot 評估。它只新增 Go 漏洞掃描與依賴更新排程，不重寫既有 race、跨平台、lint、coverage、Codecov、benchmark、Action SHA 或產品功能。

參考來源：

- Go 官方 govulncheck 文件：https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck
- Go 官方 v1.6.0 tag：https://go.googlesource.com/vuln/+/refs/tags/v1.6.0
- GitHub 官方 Dependabot 設定說明：https://docs.github.com/en/code-security/concepts/supply-chain-security/about-the-dependabot-yml-file
- GitHub 官方 Dependabot options：https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference
- 既有程式碼與設定：`.github/workflows/ci.yml`、`Makefile`、`go.mod`、`go.sum`

## 背景

專案已固定 Go 1.25.12／1.26.5、GitHub Actions 完整 SHA 與 golangci-lint 版本，並有 race、跨平台、lint、coverage、Codecov 與 benchmark CI。然而，已知漏洞只在人工執行時才會被發現，Go modules 與 GitHub Actions 也沒有固定更新排程。

Go 官方文件指出，`govulncheck` 預設查詢 Go vulnerability database，並以呼叫可達性縮小報告範圍。文字格式在發現漏洞時會非零退出；JSON、SARIF、OpenVEX 格式則不論是否發現漏洞都成功退出，因此不能直接作為 CI 阻擋條件。

GitHub 官方文件要求 Dependabot 設定置於 default branch 的 `.github/dependabot.yml`，每個 ecosystem 必須提供 `package-ecosystem`、`directory` 與 `schedule.interval`。

## 問題陳述

CI 無法自動阻擋目前工具鏈或可達依賴漏洞，也沒有機制定期提出 Go modules 與 GitHub Actions 更新。若只釘選工具版本而不持續查詢新漏洞與版本，供應鏈基線會隨時間失效。

## 目標

1. 以固定版本 `govulncheck` 掃描 Go 1.25.12／1.26.5 的 `./...`，對漏洞與資料庫錯誤採 fail closed。
2. 提供一致的本機 `make vuln` 入口，並驗證 scanner 版本，避免浮動 `latest`。
3. 新增 Dependabot weekly 排程，涵蓋 Go modules 與 GitHub Actions。
4. 將 minor／patch 更新分組降低 PR 噪音，major 更新維持獨立審查。
5. 保持現有七項 CI、最小 permissions、完整 Action SHA、Codecov 與產品行為不變。

## 非目標

1. 不升級任何 Go module、GitHub Action、Go 版本或 runtime dependency。
2. 不自動合併 Dependabot PR，不設定 reviewer、assignee、repository secret 或 branch protection。
3. 不啟用或修改 repository-level Dependabot alerts／security updates 開關。
4. 不建立私有 vulnerability database mirror、離線快照或漏洞豁免清單。
5. 不使用 JSON、SARIF 或 OpenVEX 作為本次 CI gate。
6. 不修改產品碼、公開 API、測試語意、coverage threshold 或 benchmark gate。

## 已定決策

- `govulncheck` 固定為官方 `golang.org/x/vuln/cmd/govulncheck@v1.6.0`。
- 明確使用官方即時資料庫 `https://vuln.go.dev`，不釘選資料庫快照；scanner 可重現性與漏洞資料新鮮度分開管理。
- vulnerability job 使用 Go 1.25.12／1.26.5 matrix，涵蓋兩個支援工具鏈的標準庫差異。
- 使用文字格式 `govulncheck -db=https://vuln.go.dev ./...`，保留官方非零退出語意。
- 資料庫或網路錯誤同樣阻擋 job，不在 workflow 內吞沒錯誤或自動降級；允許人工 rerun。
- `make vuln` 不加入 `make verify`，避免一般離線驗證被外部資料庫可用性綁定；CI 使用獨立 job。
- Dependabot 以 `gomod` 與 `github-actions` 兩個 ecosystem 設定，directory 均為 `/`。
- 排程為每週一 Asia/Taipei 09:00 與 09:30，錯開兩種 ecosystem；每個 ecosystem 的 version update PR 上限為 2。
- minor／patch 使用各自的 version-update group，major 仍產生獨立 PR；security update 不分組且不受 version PR 上限控制。
- 不使用 multi-ecosystem group，避免 Go modules 與 workflow SHA 更新混在同一 PR。

## 待確認項目

- 無。

## 現有行為

- CI 有七項工作，沒有 vulnerability scan。
- Makefile 沒有固定 scanner 版本或 `vuln` target。
- `.github/dependabot.yml` 不存在。
- 依賴或 Action 更新完全依賴人工發現與發 PR。

## 新行為

- CI 新增兩項 `govulncheck` matrix 工作，總數由七項增加為九項。
- 開發者可安裝固定版本 scanner 並執行 `make vuln`。
- 漏洞、scanner 執行錯誤與官方資料庫不可用都會使 vulnerability job 失敗。
- Dependabot 每週檢查 Go modules 與 GitHub Actions，minor／patch 分組、major 獨立。
- 現有 CI 工作、產品輸出及依賴版本不變。

## 影響範圍

- 使用者：無 runtime 行為變更。
- 維護者：新增漏洞處理與依賴更新 PR 工作流。
- API / CLI：公開 API 無變更；新增 `make vuln`。
- Data / Storage：無產品資料；CI 會查詢官方 vulnerability database。
- 文件 / 安裝 / 發布：README、DESIGN 補充 scanner 安裝、CI 與 Dependabot 政策。

## 使用情境

- 作為維護者，我想讓兩個支援 Go 版本都自動檢查可達漏洞，以便在合併前阻擋已知風險。
- 作為維護者，我想定期收到分組且限量的依賴更新 PR，以便控制供應鏈漂移與審查負擔。

## 驗收情境

### 情境：兩版 Go vulnerability scan 通過

- 場景：目前程式與依賴沒有 govulncheck 可達漏洞
- 測試：`make vuln`
- 假設：安裝 `govulncheck@v1.6.0`，官方資料庫可用
- 當：Go 1.25.12 與 Go 1.26.5 分別掃描 `./...`
- 那麼：兩項 job 都成功，輸出 scanner、Go 與 database 資訊

### 情境：漏洞或掃描錯誤阻擋 CI

- 場景：govulncheck 發現可達漏洞，或無法取得官方資料庫
- 測試：workflow contract 與受控失敗驗證
- 假設：使用文字格式，命令後方沒有 `|| true`
- 當：govulncheck 非零退出
- 那麼：vulnerability job 失敗，不改用永遠成功的 JSON／SARIF／OpenVEX gate

### 情境：scanner 版本不可漂移

- 場景：本機或 CI 使用錯誤的 govulncheck 版本
- 測試：`make vuln GOVULNCHECK=<wrong-version-binary>`
- 假設：Makefile 要求 v1.6.0
- 當：版本檢查不符合
- 那麼：target 在掃描前失敗並顯示固定安裝指令

### 情境：Go modules 定期更新

- 場景：Dependabot 讀取 default branch 設定
- 測試：`.github/dependabot.yml` schema 與 contract 檢查
- 假設：repository root 有 go.mod／go.sum
- 當：每週一 09:00 Asia/Taipei 執行 gomod update
- 那麼：minor／patch 進入同一 group，major 獨立，version PR 同時最多 2 個

### 情境：GitHub Actions 定期更新

- 場景：workflow 的完整 SHA 有新對應版本
- 測試：`.github/dependabot.yml` schema 與 contract 檢查
- 假設：workflow 位於 `.github/workflows/`
- 當：每週一 09:30 Asia/Taipei 執行 github-actions update
- 那麼：minor／patch 分組、major 獨立，仍由 PR CI 驗證更新後完整 SHA

### 情境：既有 CI 與產品行為不變

- 場景：新增安全自動化後執行完整品質驗證
- 測試：`make verify`、`go test -race -count=1 ./...`
- 假設：沒有修改產品碼或 dependency
- 當：執行本機與遠端 CI
- 那麼：既有七項工作全部通過，另有兩項 vulnerability job 通過

## 驗收條件

1. Makefile 固定 v1.6.0、提供 `vuln` target 與 help，版本缺失或不符時非零退出。
2. `.github/workflows/ci.yml` 新增 Go 1.25.12／1.26.5 vulnerability matrix，沿用固定 checkout／setup-go SHA、明確 runner、timeout 與 `contents: read`。
3. vulnerability job 安裝固定 scanner、顯示版本並以文字格式掃描官方即時 database；不得使用浮動版本或吞沒退出碼。
4. `.github/dependabot.yml` 使用 version 2，正確設定 `gomod`、`github-actions`、weekly、Asia/Taipei、PR limit、groups 與 commit-message prefix。
5. minor／patch group 明確限定 `applies-to: version-updates`；security updates 不誤宣稱受 `open-pull-requests-limit` 控制。
6. `go.mod`、`go.sum` 與現有 Action SHA 無變化，沒有新 secret 或額外 permissions。
7. README、DESIGN 說明 scanner 固定、database 動態、fail-closed、人工 rerun 與 Dependabot 審查政策。
8. 本機兩版掃描、完整 race、`make verify`、YAML parse、靜態 contract 與 `git diff --check` 通過。
9. 遠端九項 CI 全部通過；Dependabot 設定合併 default branch 後才開始生效，不以立即產生 PR 作驗收條件。

## 驗證需求

- Vulnerability：Go 1.25.12／1.26.5 分別執行 `make vuln`
- Unit / Integration：兩版 `go test -race -count=1 ./...`
- CLI / Dry-run：缺工具、錯版本與正確版本三種 `make vuln` 路徑
- YAML：解析 workflow 與 dependabot.yml，檢查必要欄位及型別
- 靜態安全：無 `latest`、`@main`、`|| true`、新增 secret、`pull_request_target` 或額外 permissions
- 文件檢查：README、DESIGN、requirements、design、tasks 契約一致
- 回歸驗證：`make verify`、現有七項 CI 與 dependency diff

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | 官方 vulnerability database 暫時不可用會阻擋 CI | fail closed，允許人工 rerun；不靜默略過 |
| 風險 | 即時 database 使相同 commit 在不同日期得到不同結果 | 視為安全資料更新的預期行為，記錄 scanner／Go／DB 資訊 |
| 風險 | Dependabot 設定只有合併 default branch 後才完整驗證 | PR 先做 YAML 與 contract 檢查，合併後觀察 GitHub 狀態 |
| 風險 | grouped PR 隱藏單一依賴風險 | minor／patch 才分組，major 與 security update 分開審查 |
| 假設 | repository-level Dependabot 功能可用 | 本 spec 只提交設定；若 GitHub 權限或方案阻擋，另行處理外部設定 |

## 摘要

- 關鍵決策：固定 govulncheck v1.6.0、使用官方即時 DB、兩版 fail-closed；Dependabot weekly 且分 ecosystem
- 待確認項目：無
- 風險：外部 DB 可用性與 default branch 後才生效，以 rerun、靜態 contract 與合併後觀察處理
- 完成狀態：PR #19 已合併，九項 CI 與兩版 vulnerability log 均通過；Dependabot
  設定已進入 default branch，實際排程與更新 PR 依 GitHub 後續執行狀態觀察
