# 需求文件：修正 Windows 測試檔案 handle 洩漏

## 來源

- Type：BugFix
- Owner：待確認
- Status：InProgress
- 失敗紀錄：PR #4，GitHub Actions run `30425061820`，Windows job `90489619085`

## 文件定位

本 spec 接續 `.specs/2026-07-29-11-48_Chore-go-toolchain-ci-baseline/` 的 T9 跨平台驗證。Go 1.25.11／1.26.5 race、macOS、lint、coverage 與 benchmark 均通過，但 Windows 2025 因測試結束時仍持有日誌檔案而失敗。

本 spec 已由使用者另行指示進入實作；本機驗證完成，遠端 Windows 2025 驗收需在 commit 與 push 後執行。

## 背景

Windows job 的七個測試在 `t.TempDir()` cleanup 階段失敗，錯誤均為檔案仍被程序使用：

- `TestInitLogger_WithFileOutput`
- `TestInitLogger_WithFileAndConsoleOutput`
- `TestInitLogger_WithAllOptions`
- `TestBuildFileCore_JSONFormat`
- `TestBuildFileCore_ConsoleFormat`
- `TestBuildFileCore_WithFileName`
- `TestBuildFileCore_EmptyLogPath`

前三個測試透過 `initLogger` 發布全域 logger，只在測試開始前呼叫 `resetGlobalState`，沒有在測試結束時關閉本次建立的 file output。後四個測試呼叫 `buildFileCore`，但該 package-private helper 丟棄 `newFileCore` 回傳的 `*os.File`，使呼叫端無法履行關閉責任。

Unix-like 系統允許刪除仍開啟的檔案，因此 Linux 與本機 macOS 未顯示錯誤；Windows 禁止刪除仍被程序持有的檔案，讓真正的測試資源生命週期缺口顯現。

## 問題陳述

file-output 測試沒有明確持有並關閉所建立的檔案資源，造成測試結果依賴作業系統刪除開啟檔案的語意，Windows 2025 CI 因此無法完成跨平台驗收。

## 目標

1. 所有透過 `initLogger` 建立 file output 的測試，在 `t.TempDir()` cleanup 前關閉全域 logger 資源。
2. `buildFileCore` 不再丟棄 `*os.File`，測試呼叫端可明確取得並關閉 owned resource。
3. 測試 cleanup 使用 `t.Cleanup`，即使測試中途 `Fatal` 仍會執行。
4. cleanup 註冊順序必須保證 logger file close 早於 `t.TempDir()` RemoveAll。
5. 關閉錯誤不得被靜默忽略；測試需回報 cleanup error。
6. Windows 2025 的既有七個失敗測試全部通過。
7. Go 1.25.11／1.26.5 race、macOS、lint、coverage 與 benchmark 不回歸。
8. 不修改公開 API、runtime file ownership、日誌格式或安全邊界。
9. Windows job 通過後，將前置工具鏈 spec 的 T9 與整體狀態更新為完成。

## 非目標

1. 不在本 spec 採用 `os.Root` 或改寫 safe-open 實作。
2. 不修改 `Instance.Close`、`Configure` cleanup 或 SplitOutput runtime 契約。
3. 不以 `runtime.GOOS == "windows"` skip 失敗測試。
4. 不加入 sleep、retry 或手動延遲規避 handle 尚未釋放的問題。
5. 不修改 GitHub Actions matrix、runner label、Action SHA 或 coverage 門檻。
6. 不重構 encoder、SQL core、Context fields 或全域 logger 架構。
7. 不新增 dependency。

## 已定決策

- 這是測試與 package-private 測試入口的資源所有權修正，不是 Windows 專用 workaround。
- `initLogger` file-output 測試在建立 `t.TempDir()` 後註冊全域 cleanup；LIFO 順序使 logger 先關閉、目錄後刪除。
- `buildFileCore` 簽章調整為同時回傳 core 與 owned `*os.File`；呼叫端用 `t.Cleanup` 關閉。
- 保留 `buildFileCore` 的既有預設 `./logs` fallback 與 panic-on-build-error package-private 行為，本批只補資源所有權。
- 測試 cleanup helper 必須檢查 `Close`／global cleanup error，不能只寫 `_ = close()`。
- 不新增平台 skip；Windows job 是必要驗收證據。

## 待確認項目

- 無。

## 現有行為

- file-output init 測試結束時仍保留 `globalCleanup` 與 open file。
- `buildFileCore` 丟棄 `*os.File`，呼叫端只有 core，無法 Close。
- Linux／macOS 測試因 unlink 語意通過。
- Windows `t.TempDir` cleanup 無法刪除被占用的日誌檔，job 失敗。
- 前置 CI spec 的 T9 保持未完成。

## 新行為

- file-output init 測試無論成功、Error 或 Fatal 路徑，都在 TempDir cleanup 前回收全域 logger。
- buildFileCore 測試明確持有並關閉其 file。
- cleanup 錯誤成為測試失敗，不再被忽略。
- 七個既有測試在 Windows、macOS、Linux 行為一致。
- Windows job 通過後，前置 CI spec 完成跨平台驗收。

## 影響範圍

- 產品 API：無
- Package-private：`buildFileCore` 回傳值增加 owned `*os.File`
- 測試：global reset helper、其跨檔呼叫點、三個 init file tests、四個 build file core tests
- CI：不改 workflow，只重新驗證既有 Windows job
- 文件：本 spec；遠端通過後更新前置 spec T9

## 使用情境

- 作為維護者，我希望所有測試明確關閉自己建立的檔案，避免測試結果依賴平台 unlink 語意。
- 作為 Windows 使用者，我希望專案測試能在 Windows 2025 正常完成，而不是以 skip 隱藏資源問題。
- 作為 library 使用者，我希望修正不改變任何公開 API 或 runtime logger 行為。

## 驗收情境

### 情境：全域 file logger 早於 TempDir 關閉

- 測試：`TestInitLogger_WithFileOutput`、`TestInitLogger_WithFileAndConsoleOutput`、`TestInitLogger_WithAllOptions`
- 假設：測試透過 `t.TempDir()` 建立 file output
- 當：測試結束並執行 cleanup
- 那麼：global cleanup 先關閉 file，TempDir RemoveAll 成功，沒有 handle-in-use 錯誤

### 情境：buildFileCore 所有權可見

- 測試：四個 `TestBuildFileCore_*`
- 假設：helper 建立 file-backed core
- 當：呼叫 helper
- 那麼：同時取得 non-nil core 與 owned file，並由 `t.Cleanup` 關閉 file

### 情境：測試中途失敗仍清理

- 測試：cleanup registration review 與既有 failure paths
- 假設：資源建立後的 assertion 呼叫 `Fatal`
- 當：測試提前返回
- 那麼：`t.Cleanup` 仍關閉 global logger 或 owned file

### 情境：Windows 2025 完整測試通過

- 測試：GitHub Actions portability job
- 假設：runner 為 `windows-2025`、Go 1.26.5
- 當：執行 `go test -count=1 ./...`
- 那麼：所有測試通過，TempDir cleanup 沒有檔案占用錯誤

### 情境：既有公開行為不回歸

- 測試：完整 race、lint、coverage、benchmark
- 假設：只修改測試資源 ownership 與 package-private helper 回傳值
- 當：執行 `make verify`
- 那麼：公開 API、輸出、安全規則、Close 與 routing 行為不變

## 驗收條件

1. 七個原失敗測試在 Windows 2025 通過。
2. file-output init 測試均註冊結束 cleanup，且註冊順序晚於 TempDir cleanup。
3. 四個 buildFileCore 測試均明確關閉回傳 file。
4. 測試 cleanup 沒有 `_ = Close()` 或忽略 global cleanup error。
5. 不新增 Windows skip、sleep 或 retry。
6. `go test -race -count=1 ./...` 在 Go 1.25.11、1.26.5 通過。
7. macOS 15、Windows 2025 portability jobs 通過。
8. `make verify`、`git diff --check` 通過，coverage 維持不低於 90%。
9. workflow、go.mod、go.sum、公開 API 與 runtime file lifecycle 未變更。
10. 前置 CI spec 的 T9 只有在遠端 Windows job 真正通過後才能打勾。

## 驗證需求

- Targeted：七個原失敗測試連續執行 20 次
- Race：Go 1.25.11 與 1.26.5 完整 race
- Cross-platform：GitHub-hosted macOS 15、Windows 2025
- Quality：fmt-check、vet、golangci-lint v2.12.2、coverage、benchmark
- Boundary：沒有 Windows skip、sleep、retry、dependency 或 workflow 變更

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | t.Cleanup 註冊早於 TempDir，導致 LIFO 順序反轉 | TempDir 建立後才註冊 logger cleanup；code review 檢查順序 |
| 風險 | 同一 file 被重複關閉 | cleanup ownership 單一化；必要時接受 `os.ErrClosed` 需有明確理由 |
| 風險 | 只在 Windows skip 掩蓋問題 | 驗收禁止新增 skip |
| 風險 | package-private 簽章調整誤傷 runtime | rg 證明只由 core_test.go 使用；完整回歸 |
| 假設 | Windows 失敗皆由 open handle 引起 | GitHub failure log 七處均為 TempDir unlinkat handle-in-use |

## 摘要

- 根因：測試未在 TempDir cleanup 前關閉 file handle
- 修正：global cleanup 排序 + buildFileCore 回傳 owned file
- 邊界：不改公開 API、runtime lifecycle、CI 或 os.Root
- 下一步：審閱 design.md 與 tasks.md；未經使用者指示不得開始實作
