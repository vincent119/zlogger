# 需求文件：SplitOutput 與 Context 效能基線

## 來源

- Draft: 無
- Type: Chore
- Owner: vincent119
- Status: Complete

## 文件定位

本 spec 接續 `_workspace/03_performance_review.md` 與 `.specs/2026-07-29-10-52_Refactor-config-init-contract/tasks.md` 尚未完成的並行 benchmark、鎖競爭及 Context 配置量測。既有 CI 基線只涵蓋停用等級與一般 JSON fields，本 spec 補齊尚未量測的熱路徑，不重寫已完成的 SplitOutput lifecycle、Context defensive copy、檔案安全或權限選項。

參考來源：

- 需求來源：`_workspace/03_performance_review.md` 的 SplitOutput 單鎖與 Context 配置建議
- 既有文件：`DESIGN.md` 的效能與 benchmark 說明
- 既有程式碼：`benchmark_test.go`、`context.go`、`split_output.go`

## 背景

目前已有 `BenchmarkLoggerInfoDisabled` 與 `BenchmarkLoggerInfoFields`，CI 也會執行 `BenchmarkLogger` 冒煙測試。然而，Context 日誌每次合併欄位時會配置新 slice，逐次 `WithContext` 會重複複製既有欄位；SplitOutput 則以單一 mutex 保護三個 writer，且 `Write` 在鎖內完成 I/O。這些設計已具正確性測試，但缺少可重現的配置與鎖競爭數據。

## 問題陳述

專案無法量化 Context 熱路徑的 `allocs/op`，也無法判斷 SplitOutput 單鎖在並行寫入時的競爭程度。因此，任何快取、拆鎖或緩衝方案都缺少可比較基線，容易以增加複雜度換取未經證實的效益。

## 目標

1. 建立 Context 日誌及 Context 欄位建構的 `ns/op`、`B/op`、`allocs/op` 基線。
2. 建立 SplitOutput 串行及 `B.RunParallel` 寫入基線，隔離 mutex 成本，不受磁碟速度與每日換檔 worker 影響。
3. 以 mutex profile 確認 `SplitOutput.Write` 的競爭位置，並以 Go 1.25.11／1.26.5 同一提交樣本記錄工具鏈差異。
4. 讓新增 benchmark 納入既有 `make bench` 與 CI 冒煙測試，不新增 runtime dependency。

## 非目標

1. 不拆分 SplitOutput mutex、不改 writer ownership、rotation、Close 或 Sync。
2. 不加入 `zapcore.BufferedWriteSyncer`、非同步 queue、buffer pool 或快取 logger。
3. 不修改 Context API、欄位複製規則、global logger 契約或公開函式簽章。
4. 不設定未經長期樣本支持的效能 gate，不以不同硬體結果作絕對比較。
5. 不修改 CI workflow、Makefile、go.mod、go.sum 或 dependency。

## 已定決策

- 所有新增 benchmark 名稱以 `BenchmarkLogger` 開頭，直接納入現有 `BENCH_PATTERN=BenchmarkLogger`。
- Context 基線涵蓋直接日誌、只有 Context 欄位、Context 加呼叫欄位，以及 1／5／20 個欄位的批次與逐次建構。
- SplitOutput 基線使用 test-only、無狀態且可並行的記憶體 sink，避免檔案系統、Sync、Close 與 worker 污染鎖競爭數據。
- SplitOutput 至少量測串行、同 level 並行與混合 level 並行；固定 payload 並使用 `SetBytes`。
- benchmark 只建立基線與診斷證據；若數據支持產品碼調整，另立 Refactor spec。
- 不對 `ns/op` 設固定門檻；同一提交、同一硬體、固定參數與多次樣本才可比較。

## 待確認項目

- 無。

## 現有行為

- `make bench` 只執行名稱符合 `BenchmarkLogger` 的兩個既有 benchmark。
- Context 日誌與 Context 建構沒有獨立配置基線。
- SplitOutput 沒有串行、並行或 mutex profile 基線。
- CI benchmark job 只作固定 100 次的 smoke test，不作跨執行結果 gate。

## 新行為

- `make bench` 會連同既有案例執行 Context 與 SplitOutput benchmark。
- 開發者可使用固定 selector 收集兩版 Go 的多次樣本及 SplitOutput mutex profile。
- DESIGN 記錄量測方法、解讀限制與後續重構的決策條件。
- logger 公開行為、輸出內容與檔案生命週期完全不變。

## 影響範圍

- 使用者：無直接行為變更；維護者獲得可重現的效能資料。
- 功能：benchmark 與效能診斷。
- API / CLI：公開 API 無變更；沿用 `make bench`。
- Data / Storage：無；SplitOutput benchmark 不使用磁碟。
- 文件 / 安裝 / 發布：更新 DESIGN 與本 spec，不影響安裝或發布。

## 使用情境

- 作為維護者，我想量測 Context 配置與 SplitOutput 鎖競爭，以便用數據決定是否值得進行效能重構。
- 作為 PR 審查者，我想在相同提交與固定參數下比較 benchmark，以便區分程式碼退化、工具鏈差異與環境雜訊。

## 驗收情境

### 情境：Context 日誌配置基線

- 場景：比較直接日誌、Context 欄位及額外呼叫欄位
- 測試：`BenchmarkLoggerInfoContext`
- 假設：logger 與 Context 在 timer 外建立，輸出至 `io.Discard`
- 當：以 `-benchmem` 執行各子案例
- 那麼：每個案例輸出 `ns/op`、`B/op`、`allocs/op`，不包含建構 logger 的成本

### 情境：Context 欄位建構規模

- 場景：比較 1／5／20 個欄位的批次與逐次加入
- 測試：`BenchmarkLoggerWithContext`
- 假設：每次 iteration 從相同 base context 開始
- 當：分別以單次 variadic 呼叫及逐欄位呼叫建立 Context
- 那麼：結果可顯示欄位數量與重複複製對配置及時間的影響，且不改變 defensive copy 契約

### 情境：SplitOutput 並行鎖競爭基線

- 場景：比較串行、同 level 並行與混合 level 並行寫入
- 測試：`BenchmarkLoggerSplitOutputWrite`
- 假設：三個輸出使用無磁碟、無額外 mutex、可安全並行的 test-only sink
- 當：以固定 payload 執行一般迴圈與 `B.RunParallel`
- 那麼：輸出 `ns/op`、`B/op`、`allocs/op`、bytes 指標，且量測路徑包含真正的 `SplitOutput.Write` mutex

### 情境：mutex profile 定位

- 場景：收集 SplitOutput 並行 benchmark 的 mutex profile
- 測試：`BenchmarkLoggerSplitOutputWrite/parallel`
- 假設：只執行目標 benchmark，profile 寫入暫存目錄
- 當：使用 `-mutexprofile` 並以 `go tool pprof -top` 檢視
- 那麼：報告能辨識 `SplitOutput.Write` 的鎖競爭，產物不進入 Git

### 情境：既有行為不被破壞

- 場景：產品碼與公開契約保持不變
- 測試：`go test -race -count=1 ./...`
- 假設：差異只包含 benchmark、DESIGN 與本 spec
- 當：執行完整品質驗證
- 那麼：所有既有測試、race、coverage、lint 與 benchmark smoke 通過，且產品 `.go` 檔沒有修改

## 驗收條件

1. 新增 `BenchmarkLoggerInfoContext`、`BenchmarkLoggerWithContext`、`BenchmarkLoggerSplitOutputWrite`，全部呼叫 `ReportAllocs`。
2. SplitOutput 並行案例使用 `B.RunParallel`，固定 payload 並呼叫 `SetBytes`，不執行磁碟、網路、sleep 或每日換檔 goroutine。
3. Context benchmark 的 logger、欄位與基礎 Context 均在 timer 外準備；需要逐次建構的資料只保留被量測操作。
4. `make bench` 無需修改 `BENCH_PATTERN` 即可發現全部新增案例。
5. Go 1.25.11 與 Go 1.26.5 在同一提交、同一硬體、固定 `-count=10` 產生可供 benchstat 比較的樣本。
6. mutex profile 可定位 SplitOutput 寫入鎖；profile 與 benchmark 輸出只存放暫存目錄。
7. `make verify`、兩版完整 race 與 `git diff --check` 通過，差異未包含產品碼、CI、Makefile 或 dependency。
8. DESIGN 記錄量測條件、觀察結果、限制與是否需要後續 Refactor spec；不得將單次 CI runner 數值宣稱為穩定 gate。

## 驗證需求

- Unit / Integration：`go test -race -count=1 ./...`
- Benchmark：`go test -run=NONE -bench='BenchmarkLogger(InfoContext|WithContext|SplitOutputWrite)' -benchmem -count=10 ./...`
- Mutex profile：`go test -run=NONE -bench='BenchmarkLoggerSplitOutputWrite/parallel' -benchtime=2s -mutexprofile=<temp>/split-output-mutex.out ./...`
- CLI / Dry-run：`make bench`
- 文件檢查：DESIGN、本 requirements、design、tasks 的 selector 與量測契約一致
- 回歸驗證：兩版 Go race、`make verify`、公開 API diff 與 Allowed Changes 檢查

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | CI runner 與本機頻率調整造成 `ns/op` 雜訊 | 只在同一硬體比較多次樣本；CI 僅作 smoke test |
| 風險 | test sink 自身同步遮蔽 SplitOutput mutex | sink 不保存可變狀態、不增加 mutex 或 atomic counter |
| 風險 | 直接建構 SplitOutput 測得私有結構而非完整檔案 I/O | 明確把案例定位為鎖成本基線，既有整合測試繼續保護真實 I/O |
| 風險 | benchmark 修改全域 logger 污染其他案例 | timer 外保存並於 Cleanup 恢復，禁止與會修改全域 logger 的 benchmark 平行執行 |
| 假設 | 現有 `BenchmarkLogger` pattern 會匹配新增名稱 | 以 `make bench` 列出的案例驗證 |

## 摘要

- 關鍵決策：先建立 Context 配置與 SplitOutput 鎖競爭基線，不在本 spec 最佳化產品碼
- 待確認項目：無
- 風險：環境雜訊與 test sink 失真，以固定參數、多次樣本和 mutex profile 降低
- 下一步：使用者確認後依 tasks 從 benchmark 缺口盤點與測試骨架開始實作
