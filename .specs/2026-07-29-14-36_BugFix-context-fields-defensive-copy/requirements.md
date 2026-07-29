# 需求文件：Context fields 邊界防禦性複製

## 來源

- Draft：無
- Type：BugFix
- Owner：待確認
- Status：Complete
- 前置規格：`.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/tasks.md` 的後續改善

## 文件定位

本 spec 接續前置安全規格所記錄的 `Context fields defensive copy` 待辦，只修正 `WithContext` 與 `FromContext` 在 `[]Field` 邊界共享底層陣列的問題。本 spec 不重寫全域 logger、`Instance`、檔案輸出、SplitOutput、encoder、SQL 處理或欄位建構 helper。

參考來源：

- 需求來源：使用者確認的下一項改善，以及專案 `AGENTS.md` 對 slice 入／出邊界一律複製的規範
- 既有文件：`DESIGN.md` 的 Context 支援設計
- 既有程式碼：`context.go`、`context_test.go`

## 背景

`WithContext` 在 context 尚無欄位時，會直接把 variadic `fields` 的 slice 存入 `context.WithValue`。呼叫端若在建立 context 後修改原 slice，已建立 context 內的欄位也會被改變。

`FromContext` 目前直接回傳 context 內保存的 slice。取得欄位的呼叫端可修改回傳 slice，進而改變後續 `InfoContext` 等函式讀到的內容。在並行使用 context 時，這兩種 aliasing 也可能形成 data race。

## 問題陳述

公開 API 的輸入與輸出 slice 沒有明確的所有權隔離，使理應以不可變方式傳遞的 context 狀態可被外部修改，並可能造成跨 goroutine 的資料競爭。

## 目標

1. `WithContext` 不保存呼叫端可繼續修改的 `[]Field` 底層陣列。
2. `FromContext` 不暴露 context 內部保存的 `[]Field` 底層陣列。
3. 保持欄位順序、累積語意、nil 行為與所有既有 Context helper 相容。
4. 內部日誌合併不因公開 API 的 defensive copy 產生不必要的雙重複製。
5. 以 deterministic tests 與 race tests 覆蓋輸入、輸出兩側的 mutation。

## 非目標

1. 不深拷貝 `zap.Field.Interface`、map、slice、pointer 或其他欄位值內部參照。
2. 不保證呼叫端放入 `Any` 的可變物件可安全跨 goroutine 使用。
3. 不修改 `WithRequestID`、`WithUserID`、`WithTraceID`、`WithOperation` 或 `WithComponent` 的公開簽章與 key。
4. 不修改 context 的 cancellation、deadline 或 value propagation。
5. 不處理 encoder 假契約、SQL dead code 或 README.en 同步。
6. 不新增外部 dependency。

## 已定決策

- defensive copy 的單位是 `[]Field` 與其中的 `zap.Field` 值，採淺層複製。
- 新增 package-private accessor 讀取內部 slice；只有不會把 slice 暴露給 package 外的路徑可使用。
- `FromContext` 對外回傳 clone；沒有欄位與 nil context 仍回傳 nil。
- `WithContext(ctx)` 沒有 fields 時仍回傳原 context，不額外包裝。
- 欄位累積順序維持「既有欄位在前，新欄位在後」。

## 待確認項目

- 無。

## 現有行為

- 第一次呼叫 `WithContext(ctx, fields...)` 時可能直接保存 `fields` 的底層陣列。
- `FromContext` 回傳內部 slice，修改回傳元素會污染 context。
- 已有 context 欄位時，`WithContext` 會配置合併 slice，維持欄位累積順序。
- nil context 由 `WithContext` 轉為 `context.Background()`；`FromContext(nil)` 回傳 nil。

## 新行為

- 每次實際加入欄位時，`WithContext` 都建立由新 context 獨占的 slice。
- 每次 `FromContext` 取得欄位時，都回傳與內部儲存分離的 slice。
- 修改輸入 slice 或任一輸出 slice，不會改變後續從 context 取得或寫入日誌的欄位。
- 內部合併直接讀取私有 slice，再配置一次最終輸出，避免公開 clone 後再次複製。

## 影響範圍

- 使用者：所有使用 `WithContext`、`FromContext` 與 `*Context` 日誌函式的呼叫端
- 功能：Context fields 儲存、提取與日誌合併
- API / CLI：公開函式簽章不變；`FromContext` 的回傳值改為呼叫端擁有的副本
- Data / Storage：無
- 文件 / 安裝 / 發布：更新 `DESIGN.md` 與相關 godoc；不影響安裝及發布流程

## 使用情境

- 作為 library 使用者，我想讓加入 context 的欄位不受原始 slice 後續修改影響，以便安全傳遞 request context。
- 作為 library 使用者，我想安全檢視並調整 `FromContext` 的回傳值，而不污染其他使用同一 context 的程式。
- 作為並行程式開發者，我想讓 context 欄位的 slice ownership 清楚，以避免因 API aliasing 產生 data race。

## 驗收情境

### 情境：隔離首次寫入的輸入 slice

- 場景：context 尚未保存欄位
- 測試：`TestWithContextCopiesInputFields/first_batch`
- 假設：呼叫端以既有 `[]Field` 建立 context
- 當：建立完成後修改原 slice 的欄位元素
- 那麼：`FromContext` 仍回傳建立當下的欄位內容

### 情境：隔離追加寫入的輸入 slice

- 場景：context 已有欄位後再加入一批欄位
- 測試：`TestWithContextCopiesInputFields/appended_batch`
- 假設：parent context 已保存一個欄位
- 當：以另一個 `[]Field` 建立 child context，接著修改追加 slice
- 那麼：child 保持既有與新增欄位，parent 內容不變，欄位順序不變

### 情境：隔離 FromContext 回傳 slice

- 場景：同一 context 被多次讀取
- 測試：`TestFromContextReturnsDefensiveCopy`
- 假設：context 已保存至少一個欄位
- 當：呼叫端修改第一次 `FromContext` 的回傳元素
- 那麼：第二次 `FromContext` 仍回傳原始欄位內容

### 情境：並行讀取不受外部 slice mutation 影響

- 場景：context 在多個 goroutine 間傳遞
- 測試：`TestContextFieldsDefensiveCopyConcurrentAccess`
- 假設：context 已由輸入 slice 建立，且呼叫端已取得一份輸出 slice
- 當：不同 goroutine 分別修改原輸入、修改輸出副本及持續讀取 context
- 那麼：`go test -race` 不回報由 context 欄位 slice aliasing 造成的 data race，context 內容保持不變

### 情境：既有 nil 與空欄位行為不被破壞

- 場景：nil context、空 context 與零個 fields
- 測試：`TestWithContext_NilContext|TestWithContext_NoFields|TestFromContext_NilContext|TestFromContext_NoFields`
- 假設：沿用既有呼叫方式
- 當：執行既有 API
- 那麼：回傳值與目前契約一致

### 情境：既有欄位合併與日誌輸出不被破壞

- 場景：累積多個 context fields 並附加呼叫欄位
- 測試：`TestWithContext_AddFields|TestMergeContextFields_MergeFields|TestContextLogFunctions|TestMultipleContextFields`
- 假設：既有 logger 已完成測試初始化
- 當：使用 `InfoContext` 等函式寫入日誌
- 那麼：context 欄位在前、呼叫欄位在後，key 與值維持不變

## 驗收條件

1. 輸入 slice 在 `WithContext` 回傳後被修改，不影響 context 內欄位。
2. `FromContext` 回傳 slice 被修改，不影響 context 內欄位及後續讀取。
3. parent 與 child context 不會因 slice mutation 互相污染。
4. defensive copy 僅為淺層，文件不宣稱可隔離 `Field.Interface` 內的可變物件。
5. 所有既有 Context tests 通過，公開 API、key、順序、nil 與空值行為不變。
6. `go test -race -count=1 ./...`、目標測試連續 20 次及 `make verify` 通過。
7. coverage 不低於專案既有 90% 門檻。

## 驗證需求

- Unit：`go test -count=1 -run 'Test(WithContextCopiesInputFields|FromContextReturnsDefensiveCopy|ContextFieldsDefensiveCopyConcurrentAccess)$' ./...`
- Race：`go test -race -count=1 ./...`
- 穩定性：目標 defensive copy 與既有 Context selectors 執行 `-count=20`
- 品質：`make verify`
- 文件檢查：確認 `DESIGN.md` 與 godoc 明確描述淺層複製及 ownership
- 邊界檢查：`git diff --stat`、`git diff --check`

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | 每次 `FromContext` 增加一次 slice 配置 | 私有 accessor 供內部合併使用，避免 clone 後再複製 |
| 風險 | 使用者誤以為 nested value 也會被深拷貝 | requirements、DESIGN 與 godoc 明確限定為淺層複製 |
| 風險 | 修改後改變 nil 或欄位順序 | 保留既有測試並新增明確順序驗收 |
| 假設 | `zap.Field` 值複製足以隔離 slice 元素替換 | 以 deterministic mutation tests 驗證；nested reference 列為非目標 |

## 摘要

- 關鍵決策：輸入與公開輸出均淺層複製，內部透過私有 accessor 避免雙重配置
- 待確認項目：無
- 風險：額外配置成本與 nested reference 不受保護
- 下一步：使用者確認後依 tasks.md 先建立 TDD Red tests，再修改 `context.go`
