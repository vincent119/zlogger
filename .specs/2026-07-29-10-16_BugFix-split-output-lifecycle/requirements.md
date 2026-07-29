# 需求文件：修正分級日誌生命週期與路由

## 來源

- Draft: 無
- Type: BugFix
- Owner: 待確認
- Status: Complete

## 文件定位

本 spec 接續 `_workspace/05_review_summary.md` 的第一優先修正項目，只處理 `SplitOutput` 的生命週期、換檔、分級路由、同步與相關測試。本 spec 不重寫全域 logger、Config、encoder 或一般檔案輸出，也不處理其他審查項目。

參考來源：

- 需求來源：使用者要求依 SDD 先建立需求、設計與 tasks，不執行修改
- 既有文件：`README.md`、`DESIGN.md`、`_workspace/01_style_review.md`、`_workspace/02_security_review.md`、`_workspace/03_performance_review.md`、`_workspace/04_architecture_review.md`、`_workspace/05_review_summary.md`
- 既有程式碼：`split_output.go`、`split_output_test.go`

## 背景

`NewSplitOutput` 目前會啟動無退出條件的每日換檔 goroutine。`Close` 只關閉三個檔案，無法停止 goroutine；下一次排程到期時，已關閉的輸出會重新開檔。`GetSplitCore` 的 level enabler 只允許 INFO 進入 info core，與文件宣稱 DEBUG 應寫入 info 檔的契約不一致。現有測試多數只確認建構時已建立的檔案存在，無法證明訊息實際寫入正確檔案。

此外，`splitOutputWrapper.Sync` 固定回傳 nil，未將 zap 的同步要求傳遞到底層檔案，`Close` 也忽略檔案關閉錯誤。

## 問題陳述

分級日誌輸出的資源生命週期與 cleanup 契約不可靠，可能造成 goroutine 與檔案描述元洩漏；分級路由與文件不一致，且測試無法阻止相關回歸。

## 目標

1. `Close` 可同步停止換檔 worker，且關閉後不會重新開檔。
2. `Close` 可安全重複及並行呼叫，不發生 panic、deadlock 或重複關閉競態。
3. `Close` 後的 `Write` 可預期地回傳可用 `errors.Is(err, os.ErrClosed)` 判斷的錯誤。
4. DEBUG 與 INFO 寫入 info 檔，WARN 寫入 warn 檔，ERROR 以上寫入 error 檔。
5. zap 呼叫 `Sync` 時，實際同步目前開啟的分級檔案，並回傳可觀察的錯誤。
6. 換檔失敗不應先破壞仍可用的舊檔案集合。
7. 測試應驗證實際內容、錯誤與 worker 結束，不依賴午夜等待、固定絕對路徑或 goroutine 數量猜測。

## 非目標

1. 不調整 `Config` 合併語意、`Init` panic 或全域 logger 生命週期。
2. 不實作 `NewNoEscapeJSONEncoder`、`DisableHTMLEscaping` 或 `sqlProcessingCore`。
3. 不處理 `FileName`、`filePrefix` 的路徑逸出、symlink 與檔案權限；另立安全性 spec。
4. 不加入非同步 buffer、拆分 mutex 或設定效能門檻；需先有 benchmark 證據。
5. 不變更 `NewSplitOutput`、`GetSplitCore`、`SplitOutput.Write`、`SplitOutput.Close` 的公開函式簽章。
6. 不新增外部依賴。

## 已定決策

- 保持現有公開 API 簽章相容。
- DEBUG 維持既有文件契約，寫入 info 檔。
- `Close` 為同步且冪等；回傳底層資源關閉錯誤。
- 關閉後寫入以包裝 `os.ErrClosed` 的錯誤表示，不新增 exported sentinel error。
- 換檔排程需具備內部可替換的時間抽象，讓測試可決定性觸發 timer 與關閉流程。
- 正式實作不得以 `runtime.NumGoroutine` 或實際等待午夜作為主要驗收方式。

## 待確認項目

- 無。若實作時發現保持公開簽章無法滿足驗收條件，必須先更新本 spec 並取得使用者確認。

## 現有行為

- 每個 `NewSplitOutput` 永久啟動一個 `rotateDaily` goroutine。
- `Close` 關檔後，worker 仍可能於午夜重新開檔。
- `GetSplitCore` 會丟棄 DEBUG，雖然 `SplitOutput.Write` 與註解將 DEBUG 對應至 info。
- `splitOutputWrapper.Sync` 不執行任何同步。
- 路由測試多數只斷言檔案存在，未驗證內容與排他性。

## 新行為

- 建構成功後啟動一個可取消 worker；建構失敗不留下 goroutine 或已開啟檔案。
- `Close` 標記輸出已關閉、通知 worker、等待 worker 結束，再完成所有檔案清理。
- 第二次及後續 `Close` 回傳第一次關閉的穩定結果，不重複執行清理。
- 已關閉的輸出不接受寫入、同步或換檔，也不會重新建立檔案。
- 換檔先完整開啟新檔案集合，成功後才原子替換；失敗時保留舊檔案可寫。
- DEBUG、INFO、WARN、ERROR、DPANIC、PANIC、FATAL 依既定規則進入且只進入對應檔案。

## 影響範圍

- 使用者：使用 `NewSplitOutput` 或 `GetSplitCore` 的 library 呼叫端
- 功能：分級日誌、每日換檔、cleanup、zap Sync
- API / CLI：公開簽章不變；補強既有行為契約
- Data / Storage：既有檔名格式不變；修正訊息路由與關閉後重新開檔問題
- 文件 / 安裝 / 發布：更新 README 與 DESIGN 的 cleanup、DEBUG 路由與 Sync 說明；不變更安裝流程

## 使用情境

- 作為服務維運者，我想要 logger cleanup 真正結束所有背景工作與檔案資源，以便應用可預期地優雅關機。
- 作為 library 使用者，我想要各級別日誌只出現在文件指定的檔案，以便監控與告警不遺漏 DEBUG 或混入錯誤級別。
- 作為維護者，我想要決定性的生命週期測試，以便後續修改不會重新引入資源洩漏。

## 驗收情境

### 情境：Close 停止 worker 且不重新開檔

- 場景：關閉已啟動每日換檔的分級輸出
- 測試：`TestSplitOutputCloseStopsRotation`
- 假設：使用可控制 timer 的內部測試建構式建立 `SplitOutput`
- 當：呼叫 `Close`，等待其回傳，再觸發原排程 timer
- 那麼：worker 已結束、不建立新日期檔案，且測試不需等待真實時間

### 情境：Close 可重複及並行呼叫

- 場景：多個 goroutine 同時清理同一輸出
- 測試：`TestSplitOutputCloseIdempotent`
- 假設：`SplitOutput` 已成功開啟三個檔案並啟動 worker
- 當：兩個以上 goroutine 同時呼叫 `Close`
- 那麼：所有呼叫均完成、沒有 panic 或 deadlock，底層資源只清理一次，race detector 無報告

### 情境：關閉後拒絕寫入與同步

- 場景：呼叫端誤用已關閉輸出
- 測試：`TestSplitOutputAfterClose`
- 假設：`Close` 已成功回傳
- 當：呼叫 `Write` 與 `Sync`
- 那麼：兩者都回傳可由 `errors.Is(err, os.ErrClosed)` 識別的錯誤，且不建立或重新開啟檔案

### 情境：分級核心正確路由內容

- 場景：透過 `GetSplitCore` 寫入所有 zap level
- 測試：`TestGetSplitCoreRoutesLevels`
- 假設：使用 `t.TempDir()` 建立輸出並取得 core 與 cleanup
- 當：依序記錄 DEBUG、INFO、WARN、ERROR、DPANIC、PANIC、FATAL 測試訊息，執行 Sync 與 cleanup
- 那麼：DEBUG/INFO 只在 info 檔、WARN 只在 warn 檔、ERROR 以上只在 error 檔，所有訊息各出現一次

### 情境：換檔失敗保留舊輸出

- 場景：下一日期的新檔案集合無法完整開啟
- 測試：`TestSplitOutputRotationFailureKeepsCurrentFiles`
- 假設：現有檔案集合可寫，時間與開檔行為可由內部依賴控制
- 當：觸發換檔且新檔案開啟失敗
- 那麼：舊檔案仍可接受寫入，已部分開啟的新檔案已清理，worker 可等待下一次排程或關閉

### 情境：既有檔名與級別行為不被破壞

- 場景：建立分級輸出並寫入 INFO、WARN、ERROR
- 測試：`TestSplitOutputFileNames`、`TestGetSplitCoreRoutesLevels`
- 假設：directory 與 prefix 合法
- 當：建立輸出並寫入既有支援級別
- 那麼：仍產生 `{prefix}-info-{date}.log`、`{prefix}-warn-{date}.log`、`{prefix}-error-{date}.log`

## 驗收條件

1. 上述六個驗收情境均由穩定、可重複執行的測試覆蓋。
2. `go test -race -count=1 ./...` 通過，且並行 Close 情境無競態。
3. `go test -count=20 -run 'TestSplitOutput|TestGetSplitCore' ./...` 通過，降低生命週期測試偶發失敗風險。
4. `go vet ./...`、`gofmt -d *.go`、`golangci-lint run ./...` 通過。
5. 公開 API 簽章與檔名格式不變。
6. README、DESIGN 與 godoc 對 DEBUG、cleanup、換檔與 Sync 的描述一致。

## 驗證需求

- Unit / Integration：`go test -count=1 -run 'TestSplitOutput|TestGetSplitCore' ./...`
- CLI / Dry-run：無
- 文件檢查：核對 `README.md`、`DESIGN.md` 與 `split_output.go` 的契約一致性
- 回歸驗證：`go test -race -count=1 ./...`

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | Close 與換檔同時發生造成 deadlock | 固定鎖、停止訊號、等待 worker、關檔的順序，加入並行測試 |
| 風險 | 新舊檔案交換期間遺失或寫入已關閉檔案 | 新集合完整開啟後，在短臨界區內交換；離開臨界區後關閉舊集合 |
| 風險 | 測試時間抽象滲漏到公開 API | 僅使用 package-private interface、factory 與建構式 |
| 風險 | cleanup 無 error 回傳，無法傳出 Close 錯誤 | 保留既有 `func()` 簽章；直接使用 `SplitOutput.Close` 時可取得錯誤，文件說明 cleanup 的限制 |
| 假設 | DEBUG 應寫入 info 檔 | 依現有型別註解與直接 Write 行為固定為契約 |

## 摘要

- 關鍵決策：保持公開 API，相容地修正可停止 worker、原子換檔、冪等 Close、實際 Sync 與 DEBUG 路由
- 待確認項目：無
- 風險：Close、換檔與 Write 的鎖順序，以及測試時間抽象的範圍
- 下一步：審閱 `design.md` 與 `tasks.md`；未經使用者指示不得進入實作
