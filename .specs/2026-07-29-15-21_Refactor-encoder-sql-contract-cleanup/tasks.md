# 任務文件：Encoder 契約與 SQL dead code 清理

Status: Complete

## Execution Context

- 意圖：讓 encoder helper 契約與 zap 實際行為一致，移除無作用 hook、SQL dead code 與不存在的文件宣稱
- 本輪授權：只建立 requirements、design、tasks；使用者再次明確指示後才進入產品碼與測試實作
- 非目標：不移除 v1 匯出符號、不新增 encoder 或 SQL 功能、不修改 logger factory、Config、Context、輸出、安全邊界或 dependency
- 已定決策：兩個匯出 helper 保留並 deprecated；`DisableHTMLEscaping` 改為 identity no-op；SQL private code/tests 移除；DESIGN 改為資料保真
- 邊界：實作限於 `encoder.go`、`encoder_test.go`、`core.go` SQL dead code、`core_test.go` 專屬 tests、`DESIGN.md` 與本 spec
- 關鍵檔案：`encoder.go`、`encoder_test.go`、`core.go`、`core_test.go`、`DESIGN.md`
- 完成條件：behavior／Red tests、dead code 搜尋、Go 1.25.11／1.26.5 race、20 次穩定性、`make verify`、coverage >= 90%、遠端七項 CI 與文件回填全部通過

### Protected Behavior

- v1 匯出函式名稱與簽章保留
- `NewNoEscapeJSONEncoder` 仍委派 pinned zap JSON encoder
- logger factory、format、level、caller、time、stacktrace 與輸出生命週期不變
- message 與 fields 不做 SQL-specific 隱含改寫
- Config、Context、file output、SplitOutput、安全邊界與 dependency 不變
- 不宣稱 deprecated helper 能修改任意既有 logger encoder

### 邊界

#### Allowed Changes

- `encoder.go`
- `encoder_test.go`
- `core.go` 僅 SQL dead code
- `core_test.go` 僅 SQL dead-code tests
- `DESIGN.md` 的第 6 節與 zap 差異表 SQL row
- `.specs/2026-07-29-15-21_Refactor-encoder-sql-contract-cleanup/`
- 遠端驗收後，`.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/tasks.md` 僅更新 encoder／SQL 後續項目

#### Forbidden

- logger factory、Config、Context、file output、SplitOutput、security 或其他產品邏輯
- README、README.en、公開函式簽章或匯出符號移除
- `go.mod`、`go.sum`、CI、Makefile、lint 或 coverage 設定
- zap fork、自訂 JSON encoder、SQL formatter／parser／redaction
- sleep、retry、全域 test hook 或平台整組 skip

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 建立 encoder characterization 與 no-op Red tests | 無 | Complete | HTML／zap parity Green；identity／nil-safe Red |
| T2 實作 encoder compatibility deprecation | T1 | Complete | 保留符號、deprecate 並移除無作用 hook |
| T3 移除 SQL dead code 與專屬 tests | T2 | Complete | private symbols 與專屬 tests 已移除 |
| T4 修正 DESIGN 契約 | T2、T3 | Complete | DESIGN 已改為 encoder ownership 與資料保真 |
| T5 本機完整驗證與邊界檢查 | T3、T4 | Complete | 兩版 race、20 次、verify、92.8% coverage |
| T6 遠端跨平台驗收 | T5 | Complete | PR #12 的七項 CI 全部通過 |
| T7 回填完成狀態 | T6 | Complete | 本 spec 與前置待辦已結案 |

## 實作任務

- [x] T1 建立 encoder characterization 與 no-op Red tests
  - Status: Complete
  - Boundary:
    - Allowed Changes：`encoder_test.go`、本 tasks Implementation Notes
    - Forbidden：產品實作、SQL tests、sleep、只檢查 non-nil 的弱斷言
  - Depends：無
  - Context：HTML 與 zap parity 應保存現行 Green；identity 與 nil-safe 對舊 hook 實作形成 deterministic Red。
  - Verify：
    - `go test -count=1 -run 'Test(NewNoEscapeJSONEncoder|DisableHTMLEscaping)' ./...`
    - Red 必須明確顯示 logger pointer 不同或 nil panic，不接受模糊失敗
    - JSON 驗證需檢查 encode error、有效 JSON 與 HTML 原字元

- [x] T2 實作 encoder compatibility deprecation
  - Status: Complete
  - Boundary:
    - Allowed Changes：`encoder.go`、`encoder_test.go`、本 tasks Implementation Notes
    - Forbidden：匯出符號移除、簽章變更、自訂 encoder、zap dependency
  - Depends：T1
  - Context：加入 `Deprecated:` replacement；保留 JSON wrapper；`DisableHTMLEscaping` 直接 `return log`，不新增 hook。
  - Verify：
    - `go test -race -count=1 -run 'Test(NewNoEscapeJSONEncoder|DisableHTMLEscaping)' ./...`
    - `go test -count=20 -run 'Test(NewNoEscapeJSONEncoder|DisableHTMLEscaping)' ./...`
    - `go doc . NewNoEscapeJSONEncoder`
    - `go doc . DisableHTMLEscaping`

- [x] T3 移除 SQL dead code 與專屬 tests
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core.go` 的 `sqlProcessingCore`／`processSQLString`、`core_test.go` 對應 tests、本 tasks Implementation Notes
    - Forbidden：`parseLevel`、其他 core、其他 tests、SQL 替代實作
  - Depends：T2
  - Context：private SQL symbols 沒有產品呼叫點；移除 type、三個 methods、helper 與四組直接 tests，保留 `strings` import。
  - Verify：
    - `rg -n 'sqlProcessingCore|processSQLString' --glob '*.go' .` 無結果
    - `go test -race -count=1 ./...`
    - `git diff -- core.go core_test.go` 只包含指定 dead code

- [x] T4 修正 DESIGN 契約
  - Status: Complete
  - Boundary:
    - Allowed Changes：`DESIGN.md` 第 6 節與 zap 差異表 SQL row、本 tasks Implementation Notes
    - Forbidden：README、README.en、其他 DESIGN 章節重寫
  - Depends：T2、T3
  - Context：第 6 節改述 zap encoder、deprecated helpers 與資料保真；移除「自動清理 SQL」差異 row，不重新編排全文件章節。
  - Verify：
    - `rg -n 'Deprecated:|資料保真|HTML|SQL' DESIGN.md encoder.go`
    - `rg -n '自動清理 SQL|sqlProcessingCore|processSQLString' DESIGN.md` 無結果

- [x] T5 本機完整驗證與邊界檢查
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過檢查修改 CI、lint、coverage、dependency 或 Boundary 外檔案
  - Depends：T3、T4
  - Context：驗證 API compatibility、字串資料保真、dead code 移除、完整回歸與差異範圍。
  - Verify：
    - `gofmt -w encoder.go encoder_test.go core.go core_test.go`
    - Go 1.25.11／1.26.5 `go test -race -count=1 ./...`
    - encoder selectors `-count=20`
    - `make verify`
    - coverage >= 90%
    - `git diff --stat`
    - `git diff --check`

- [x] T6 遠端跨平台驗收
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：workflow、skip、sleep、平台專用繞過
  - Depends：T5
  - Context：commit／push 需使用者另行授權；PR 建立後確認兩版 race、macOS、Windows、lint、coverage 與 benchmark 七項 CI。
  - Verify：`gh pr checks` 或 `gh run view` 顯示七項 jobs 全部 pass

- [x] T7 回填完成狀態
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 spec；前置安全 spec 只更新 encoder／SQL 後續項目
    - Forbidden：其他後續待辦、產品碼或工具設定
  - Depends：T6
  - Context：遠端 green 後將 requirements、tasks 與來源待辦標記 Complete，附 PR、merge commit 與 run 證據。
  - Verify：文件狀態、checkbox 與 GitHub 結果一致

## 驗證任務

- [x] V1 Encoder behavior
  - HTML 字元保持原字元，必要 JSON 字元正確 escape
  - wrapper 與 pinned zap encoder 行為一致

- [x] V2 Compatibility no-op
  - 兩個匯出符號與簽章保留
  - deprecation replacement 可由 go doc 讀取
  - 非 nil logger identity 與 nil-safe 通過

- [x] V3 SQL dead code 與文件
  - `.go` 檔不再包含 private SQL symbols
  - DESIGN 不再宣稱自動 SQL 清理
  - message 與 fields 資料保真契約明確

- [x] V4 品質與邊界
  - gofmt、vet、lint、兩版 race、coverage、benchmark 通過
  - encoder tests 連續 20 次通過
  - coverage >= 90%
  - 差異只包含 Allowed Changes
  - 無 dependency、CI、Makefile、README 或其他產品碼變更
  - `git diff --check` 通過

- [x] V5 遠端驗收
  - Go 1.25.11／1.26.5 race 通過
  - macOS 15／Windows 2025 通過
  - 靜態分析、coverage gate、benchmark smoke 通過

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 `Execution Context`
2. 目前未完成 task
3. `Protected Behavior`
4. `Implementation Notes`

不得預設掃描整個 `.specs` 目錄。若文件很大，先用標題與關鍵字定位：

```bash
rg -n "^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:" .specs/2026-07-29-15-21_Refactor-encoder-sql-contract-cleanup
```

## Implementation Notes

- 2026-07-29：PR #11 已合併為 `2ecce1b`，GitHub Actions run `30431239688` 七項檢查全部通過；本機 `main` 已 fast-forward 同步。
- 2026-07-29：從同步後 main 建立 `refactor/encoder-sql-contract-cleanup`。
- 2026-07-29：repository 已有 v1.0.2 至 v1.0.5 tags，決定不在本 spec 移除兩個既有匯出 helper。
- 2026-07-29：pinned zap v1.27.0 JSON encoder source 明確說明不做 browser／JSONP protection escaping；`NewNoEscapeJSONEncoder` 行為成立但只是冗餘 wrapper。
- 2026-07-29：`DisableHTMLEscaping` 只加入永遠回傳 nil 的 hook，無法更換既有 encoder；現有 test 只驗證回傳非 nil。
- 2026-07-29：`sqlProcessingCore`、三個 methods 與 `processSQLString` 只被 package tests 呼叫，未接入產品 factory；DESIGN 的自動 SQL 清理宣稱與實作不一致。
- 2026-07-29：決定保留並 deprecate helper、將 Disable 函式改為 identity no-op、移除 SQL dead code/tests、以資料保真契約取代錯誤文件。
- 2026-07-29：依使用者要求只完成 SDD 文件，尚未修改 encoder、core、tests 或 DESIGN。
- 2026-07-29：T1 以 table-driven tests 驗證 `<`、`>`、`&` 保留及 zap parity；舊實作的 identity test 明確失敗，nil-safe test 在 zap `WithOptions` 觸發 nil pointer panic，保存 deterministic Red。
- 2026-07-29：T2 保留兩個匯出簽章並加入 `Deprecated:` replacement；`DisableHTMLEscaping` 改為 identity no-op，目標 race、連續 20 次及 `go doc` 驗證通過。
- 2026-07-29：T3 移除 `sqlProcessingCore`、三個 methods、`processSQLString` 與四組直接 tests；所有 `.go` 檔搜尋無結果，完整 race 通過，`parseLevel` 的 `strings` 依賴保持不變。
- 2026-07-29：T4 以 encoder ownership、deprecated compatibility 與字串資料保真取代 DESIGN 的自動 SQL 清理章節，並移除 zap 差異表的錯誤 SQL row；其他章節未重寫。
- 2026-07-29：T5 的 Go 1.25.11 與 Go 1.26.5 完整 race 均通過，encoder behavior／compatibility selectors 連續 20 次通過。
- 2026-07-29：`make verify GOLANGCI_LINT=/private/tmp/zlogger-tools/golangci-lint` 使用固定 v2.12.2 通過 fmt-check、vet、lint 0 issues、race、92.8% coverage gate 與 benchmark smoke；coverage 產物已由 `make clean` 移除。
- 2026-07-29：`git diff --check` 通過；差異只包含 encoder、指定 SQL dead code/tests、DESIGN 第 6 節／SQL row 與本 spec，未修改 dependency、CI、Makefile、README 或 Boundary 外產品碼。T6 保留等待 commit／push 後的遠端驗收。
- 2026-07-29：PR #12 已合併為 `f7ae8f6`；GitHub Actions run `30432312003` 的七項檢查全部通過，包含 Go 1.25.11／1.26.5 race、macOS 15、Windows 2025、靜態分析、coverage gate 與 benchmark smoke，完成 T6 與 V5。
- 2026-07-29：完成 T7；requirements 與 tasks 狀態改為 Complete，並回填前置安全 spec 的 encoder／SQL 後續項目。產品碼與其他後續改善未修改。

## 驗證結果摘要

- 新行為驗證：通過；HTML 字元、zap parity、identity no-op、nil-safe 與 20 次穩定性均通過
- 回歸驗證：通過；Go 1.25.11／1.26.5 完整 race 與 `make verify` 通過
- 文件一致性：已移除錯誤 SQL 宣稱，DESIGN 與 deprecated godoc 對應實作
- 剩餘風險：deprecated API 需到未來 major version 才能移除

## 後續改善

- [ ] 未來 major version 移除 deprecated encoder helpers
- [ ] 決定 README.en 維護或退場策略
