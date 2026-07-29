# 任務文件：修正分級日誌生命週期與路由

Status: Planned

## Execution Context

- 意圖：修正 `SplitOutput` 無法停止 worker、關閉後資源復活、DEBUG 路由遺漏、Sync 無作用與測試驗證力不足
- 非目標：不處理 Config、全域 logger、encoder、SQL、路徑安全、檔案權限、buffer、效能拆鎖或 CI
- 已定決策：保持公開 API；使用 private clock／timer；Close 同步且冪等；關閉後操作包裝 `os.ErrClosed`；換檔採新集合成功後交換
- 邊界：只允許修改 `split_output.go`、`split_output_test.go`、`README.md`、`DESIGN.md`
- 關鍵檔案：`split_output.go`、`split_output_test.go`
- 完成條件：requirements.md 的六個驗收情境都有測試；race、重複執行、vet、格式與 lint 全數通過；文件契約一致

### Protected Behavior

- `NewSplitOutput`、`GetSplitCore`、`Write`、`Close` 公開簽章不變。
- 檔名維持 `{prefix}-{info|warn|error}-{YYYY-MM-DD}.log`。
- INFO、WARN、ERROR、DPANIC、PANIC、FATAL 的既有正確路由不變。
- 日期仍依執行環境 local timezone 計算。
- 不新增外部依賴、不修改 `go.mod`。

### 邊界

#### Allowed Changes

- `split_output.go`
- `split_output_test.go`
- `README.md`
- `DESIGN.md`

#### Forbidden

- `core.go`、`config.go`、`encoder.go`、`context.go`、`fields.go`、`zlogger.go`
- 其他產品碼與測試檔
- `go.mod`、`go.sum`、`.github/`、`Makefile`
- `_workspace/` 既有審查報告
- 公開 API 簽章與檔名格式
- 非同步 buffer、外部 rotation 套件、mutex 效能重構

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 建立生命週期驗收測試 | 無 | Planned | 先固定 Close 與 worker 契約 |
| T2 建立路由與 Sync 驗收測試 | 無 | Planned | 可與 T1 同期準備 |
| T3 實作可取消 worker 與冪等 Close | T1 | Planned | 不得改公開簽章 |
| T4 實作安全檔案集合交換與 Sync | T1、T2、T3 | Planned | 失敗時保留舊集合 |
| T5 修正 GetSplitCore level routing | T2 | Planned | DEBUG 與 INFO 只進 info |
| T6 更新公開文件 | T3、T4、T5 | Planned | 文件描述最後行為 |
| T7 完整驗證與邊界檢查 | T1 至 T6 | Planned | 不進行發布 |

## 實作任務

- [ ] T1 建立生命週期驗收測試
  - Status: Planned
  - Boundary:
    - Allowed Changes：`split_output_test.go`
    - Forbidden：產品實作與其他測試檔
  - Depends: 無
  - Context: 建立最小 private fake clock／timer 測試需求，覆蓋 worker 停止、Close 並行冪等、Close 後 Write/Sync 與換檔失敗保留舊集合。測試不得依賴實際午夜、`runtime.NumGoroutine` 或長時間 sleep。
  - Verify:
    - `go test -count=1 -run 'TestSplitOutput(CloseStopsRotation|CloseIdempotent|AfterClose|RotationFailureKeepsCurrentFiles)' ./...`
    - 實作前預期新測試失敗，但必須可編譯；實作後全部通過

- [ ] T2 建立分級路由、Sync 與決定性錯誤測試
  - Status: Planned
  - Boundary:
    - Allowed Changes：`split_output_test.go`
    - Forbidden：產品實作與其他測試檔
  - Depends: 無
  - Context: 使用 `GetSplitCore` 實際記錄各 level，cleanup 後讀取三個檔案內容；斷言每個訊息只存在於目標檔。用 `t.TempDir()` 內的一般檔案作為父路徑，取代 `/nonexistent` 權限假設。
  - Verify:
    - `go test -count=1 -run 'Test(GetSplitCoreRoutesLevels|SplitOutputSync|NewSplitOutputInvalidDirectory|GetSplitCoreInvalidDirectory)' ./...`
    - 斷言內容、排他路由與具體 error，不只檢查檔案存在

- [ ] T3 實作可取消 worker 與冪等 Close
  - Status: Planned
  - Boundary:
    - Allowed Changes：`split_output.go`
    - Forbidden：公開 API、檔名格式、其他產品檔
  - Depends: T1
  - Context: 加入 private clock／timer、stop、done、closed、closeOnce 與 closeErr。Close 不持 mutex 等待 done；worker return 前必須關閉 done。Write 與 Sync 在 closed 後回傳包裝 `os.ErrClosed` 的錯誤。
  - Verify:
    - `go test -race -count=1 -run 'TestSplitOutput(CloseStopsRotation|CloseIdempotent|AfterClose)' ./...`
    - `go test -count=20 -run 'TestSplitOutput(CloseStopsRotation|CloseIdempotent|AfterClose)' ./...`

- [ ] T4 實作原子檔案集合交換與實際 Sync
  - Status: Planned
  - Boundary:
    - Allowed Changes：`split_output.go`、`split_output_test.go`
    - Forbidden：路徑驗證、權限策略、buffer 與拆鎖效能改造
  - Depends: T1、T2、T3
  - Context: 以 private `fileSet` 集中管理三個檔案。先完整開啟新集合，再於短臨界區交換；失敗時關閉部分新檔並保留舊集合。`SplitOutput.Sync` 對目前集合執行 fsync，wrapper Sync 只做委派；多重錯誤以 `errors.Join` 回傳。
  - Verify:
    - `go test -race -count=1 -run 'TestSplitOutput(RotationFailureKeepsCurrentFiles|Sync|CloseIdempotent)' ./...`
    - `go vet ./...`

- [ ] T5 修正 GetSplitCore level routing
  - Status: Planned
  - Boundary:
    - Allowed Changes：`split_output.go`、`split_output_test.go`
    - Forbidden：encoder 格式與其他 core 行為
  - Depends: T2
  - Context: info level enabler 明確接受 DEBUG 與 INFO；WARN 只接受 WARN；error 接受 ERROR 以上。不得讓任何訊息重複進入多個 core。
  - Verify:
    - `go test -count=20 -run 'TestGetSplitCoreRoutesLevels' ./...`
    - 人工確認七個 level 各出現一次且只在目標檔案

- [ ] T6 更新公開文件與註解
  - Status: Planned
  - Boundary:
    - Allowed Changes：`README.md`、`DESIGN.md`、`split_output.go`
    - Forbidden：擴寫非本 spec 功能或變更使用範例 API
  - Depends: T3、T4、T5
  - Context: 使用繁體中文說明 DEBUG 路由、cleanup 同步結束 worker、每日換檔失敗保留舊檔、Sync 會同步檔案。修正與本段直接相關的 `timberjack` 錯字，但不進行全文件語言重寫。
  - Verify:
    - `rg -n 'DEBUG|cleanup|Close|Sync|換檔|rotation|lumberjack|timberjack' README.md DESIGN.md split_output.go`
    - 文件與 requirements.md 的新行為逐項核對

## 驗證任務

- [ ] T7 驗收情境覆蓋
  - Verify: requirements.md 的六個情境各有對應測試 selector，且不以真實時間等待或檔案存在作為唯一斷言

- [ ] T8 回歸驗證
  - Verify:
    - `go test -race -count=1 ./...`
    - `go test -count=20 -run 'TestSplitOutput|TestGetSplitCore' ./...`

- [ ] T9 品質檢查清單
  - `gofmt -d *.go` 無差異
  - `go vet ./...` 通過
  - `golangci-lint run ./...` 通過
  - 所有測試錯誤訊息與新增註解使用繁體中文
  - README、DESIGN、godoc 契約一致
  - 主要驗收情境已覆蓋
  - Protected Behavior 回歸驗證通過
  - 風險項目已處理
  - `git diff --stat` 已檢查且只含 Allowed Changes
  - `git diff --check` 已通過

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 `Execution Context`
2. 目前未完成 task
3. `Protected Behavior`
4. `Implementation Notes`

不得預設掃描整個 `.specs` 目錄。若文件很大，先用標題與關鍵字定位：

```bash
rg -n "^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:" .specs/2026-07-29-10-16_BugFix-split-output-lifecycle
```

## Implementation Notes

- 2026-07-29：完成 SDD 文件；尚未執行任何產品碼或測試修改。
- Go 工具鏈已確認為 Go 1.26.5；工具鏈升級不屬於本 spec 的實作 task。

## 驗證結果摘要

- 新行為驗證：未執行，本 spec 尚未進入實作
- 回歸驗證：未執行，本輪只建立規劃文件
- 文件一致性：需求、設計、tasks 已建立，待最終交叉檢查
- 剩餘風險：實作時的鎖順序、換檔與 Close 競爭、fake clock 最小化

## 後續改善

- [ ] 另立 Config 與 Init 公開契約 spec
- [ ] 另立檔案路徑、symlink、權限與 redaction 安全 spec
- [ ] 另立 encoder 與 SQL 死碼清理 spec
- [ ] 另立 SplitOutput benchmark 與鎖競爭評估 spec
