# 任務文件：補強 Config 與初始化契約

Status: Complete

## Execution Context

- 意圖：修正 Config 部分覆寫歧義、非法設定靜默降級、Init panic／失敗不可重試，以及一般 file output 無 cleanup 的缺失
- 本輪授權：使用者已指示 run task，依本文件順序執行 TDD、產品碼、文件與驗證
- 非目標：不處理 SplitOutput、路徑安全、encoder、SQL dead code、context、效能、CI 或 dependency
- 建議方向：新增三態 ConfigPatch、嚴格 Resolve／Validate、無副作用 New builder、可回傳 error／cleanup 的 Configure；保留 legacy API
- 已定決策：採 `ConfigPatch`、`Resolve`、`Instance`、`New`、`Configure`；成功後重複 Configure 回傳 `ErrAlreadyConfigured`；legacy Init 標示 deprecated；未提供 LogPath 使用預設，明確空字串驗證失敗
- 關鍵檔案：`config.go`、`core.go`、`config_test.go`、`core_test.go`、`README.md`、`DESIGN.md`
- 完成條件：九個驗收情境有 TDD 測試；race、重複執行、vet、格式、lint 與文件檢查通過；Protected Behavior 保持

### Protected Behavior

- `Config`、`DefaultConfig`、`Merge`、`Init` 與所有既有套件級日誌函式簽章不變。
- `Init(nil)` 仍可編譯；DefaultConfig 的值不變。
- console／json 編碼、日期檔名與 SetLevel 行為不變。
- SplitOutput、encoder 與 context API 不變。
- 不新增外部依賴，不修改 `go.mod`、`go.sum`、CI 或發布設定。

### 邊界

#### Allowed Changes

實作階段限於：

- `config.go`
- `config_test.go`
- `core.go`
- `core_test.go`
- `README.md`
- `DESIGN.md`
- 本 spec 目錄內文件

#### Forbidden

- `split_output.go`、`encoder.go`、`context.go`、`fields.go`、`zlogger.go` 及其測試
- `go.mod`、`go.sum`、`.github/`、`Makefile`
- `_workspace/` 既有審查報告
- 直接修改既有公開簽章或 Config 欄位型別
- 路徑安全、權限、redaction、SQL、buffer 與效能重構
- 未完成 T0 就新增公開 API

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T0 確認公開契約 | 無 | Complete | 已依 run task 採建議方案 |
| T1 建立 Config 解析與驗證測試 | T0 | Complete | TDD Red／Green 已記錄 |
| T2 實作 ConfigPatch、Resolve 與驗證 | T1 | Complete | race 與 20 次測試通過 |
| T3 建立 New／cleanup 驗收測試 | T0 | Complete | I/O、LIFO、並行 Close 已覆蓋 |
| T4 實作無副作用 builder 與資源 rollback | T2、T3 | Complete | 不發布全域狀態 |
| T5 建立 Configure 狀態與重試測試 | T0 | Complete | TDD Red／Green 已記錄 |
| T6 實作全域成功發布與相容層 | T4、T5 | Complete | 失敗可重試 |
| T7 更新公開文件 | T2、T4、T6 | Complete | 新舊入口與遷移已同步 |
| T8 完整驗證與邊界檢查 | T1 至 T7 | Complete | 未執行發布 |

## 實作任務

- [x] T0 確認公開契約與更新 spec
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 spec 的 `requirements.md`、`design.md`、`tasks.md`
    - Forbidden：所有產品碼、測試與公開文件
  - Depends: 無
  - Context: 與使用者確認五項決策：公開 API 名稱、New 回傳三值或具名生命週期型別、Configure 成功後重複呼叫規則、legacy Init 的 deprecated／錯誤策略、file output 空白 LogPath 規則。將結果寫入「已定決策」，清空對應待確認項目。
  - Verify:
    - `rg -n '待確認|T0|ConfigPatch|Resolve|New|Configure|Init|LogPath' .specs/2026-07-29-10-52_Refactor-config-init-contract`
    - requirements、design、tasks 對簽章與狀態轉移描述一致

- [x] T1 建立 Config 部分解析與驗證測試
  - Status: Complete
  - Boundary:
    - Allowed Changes：`config_test.go`
    - Forbidden：產品碼、其他測試與文件
  - Depends: T0
  - Context: 使用 table-driven tests 覆蓋未提供、明確 false、大小寫正規化、Outputs defensive copy、非法 Level／Format／Outputs 與 file 必要欄位。測試錯誤以 `errors.Is` 判斷分類，不綁死完整錯誤文字。
  - Verify:
    - `go test -count=1 -run 'TestConfig(PatchResolve|Validate)' ./...`
    - Red 階段允許因尚未實作 API 而編譯失敗，但必須先記錄失敗證據；Green 後全部通過

- [x] T2 實作 ConfigPatch、Resolve 與驗證
  - Status: Complete
  - Boundary:
    - Allowed Changes：`config.go`、`config_test.go`
    - Forbidden：core 建構、全域狀態、legacy API 簽章與外部依賴
  - Depends: T1
  - Context: 依 T0 確認名稱建立三態欄位；Resolve 從全新 DefaultConfig 套用非 nil 值，正規化 enum，複製 Outputs，再執行驗證。輸入 ConfigPatch、來源 slice 與 DefaultConfig 均不得被修改。
  - Verify:
    - `go test -race -count=1 -run 'TestConfig(PatchResolve|Validate)' ./...`
    - `go test -count=20 -run 'TestConfig(PatchResolve|Validate)' ./...`

- [x] T3 建立非全域建構、錯誤與 cleanup 驗收測試
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core_test.go`
    - Forbidden：產品碼、其他測試與文件
  - Depends: T0
  - Context: 覆蓋 console／file 成功建構、非法設定在 I/O 前失敗、決定性開檔錯誤不 panic、部分資源 rollback、cleanup LIFO、並行冪等與錯誤穩定。必要時以最小 package-private file opener seam 注入部分失敗，不暴露測試介面為公開 API。
  - Verify:
    - `go test -count=1 -run 'TestNew(Returns|Cleanup|RollsBack|DoesNotMutate)' ./...`
    - race detector 下 cleanup 並行情境無報告

- [x] T4 實作無副作用 builder、錯誤回傳與資源所有權
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core.go`、`core_test.go`
    - Forbidden：全域發布邏輯、其他產品檔、日誌格式與 encoder 行為
  - Depends: T2、T3
  - Context: 將 encoder/core/resource 建構與 global mutation 分離。所有可恢復錯誤立即回傳並以 `%w` 包裝；建構暫存狀態集中 closers，失敗 LIFO rollback。console 不關閉 stdout，file resource cleanup 使用 `sync.Once` 保存第一次結果。
  - Verify:
    - `go test -race -count=1 -run 'TestNew(Returns|Cleanup|RollsBack|DoesNotMutate)' ./...`
    - `go vet ./...`

- [x] T5 建立 Configure 失敗重試、成功後重複與發布測試
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core_test.go`
    - Forbidden：產品碼、其他測試與文件
  - Depends: T0
  - Context: 使用 package-private fixture 保存與恢復所有 global state。驗證失敗不改 global logger/config/zap globals 且可重試；成功後套件級記錄與 SetLevel 正常；競爭 Configure 只有一個成功，其他依 T0 回傳受控錯誤且清理候選資源。
  - Verify:
    - `go test -race -count=1 -run 'TestConfigure(CanRetryAfterFailure|RejectsSecondSuccess|PublishesAtomically|Concurrent)' ./...`
    - 測試不得 `t.Parallel()` 或依賴檔案間執行順序

- [x] T6 實作 Configure 狀態機與 legacy 相容層
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core.go`、`core_test.go`、`config.go`
    - Forbidden：既有公開簽章、其他產品檔、動態 reconfigure
  - Depends: T4、T5
  - Context: mutex 保護 Uninitialized／Building／Initialized／Closed 狀態；只有完整 New 成功後發布。失敗完成 rollback 後回到可重試狀態。Configure cleanup 只關閉本次全域 logger owned resources。依 T0 處理成功後重複呼叫與 legacy Init。
  - Verify:
    - `go test -race -count=1 -run 'Test(Configure|LegacyInit)' ./...`
    - `go test -count=20 -run 'TestConfigure(CanRetryAfterFailure|RejectsSecondSuccess|Concurrent)' ./...`

- [x] T7 更新 README、DESIGN 與 godoc
  - Status: Complete
  - Boundary:
    - Allowed Changes：`README.md`、`DESIGN.md`、`config.go`、`core.go`
    - Forbidden：擴寫其他功能、變更產品行為
  - Depends: T2、T4、T6
  - Context: 以新安全入口作建議範例，說明 ConfigPatch 的未提供／false、嚴格驗證、Sync 與 cleanup 順序、Configure 失敗重試、legacy Init 限制與遷移方式。完整程式碼範例必須處理 error 與 cleanup error。
  - Verify:
    - `rg -n 'ConfigPatch|Resolve|New\(|Configure|cleanup|Init\(|Deprecated|錯誤|重試' README.md DESIGN.md config.go core.go`
    - 文件簽章、錯誤與生命週期契約和 requirements 完全一致

## 驗證任務

- [x] T8 驗收情境覆蓋
  - Verify: requirements.md 的九個情境均有對應測試名稱與 selector，且使用決定性 I/O 失敗與狀態 fixture

- [x] T9 回歸與穩定性驗證
  - Verify:
    - `go test -race -count=1 ./...`
    - `go test -count=20 -run 'TestConfig(PatchResolve|Validate)|TestNew(Returns|Cleanup|RollsBack)|TestConfigure' ./...`

- [x] T10 品質與邊界檢查
  - `gofmt -d *.go` 無差異
  - `go vet ./...` 通過
  - `golangci-lint run ./...` 通過
  - 所有新增錯誤訊息、註解與文件使用繁體中文
  - 新 error 支援 `errors.Is`，底層 I/O error 以 `%w` 保留
  - 輸入與輸出 slice 已 defensive copy
  - 新入口沒有 library panic 或吞錯
  - README、DESIGN、godoc 契約一致
  - `git diff --stat` 只包含 Allowed Changes
  - `git diff --check` 通過

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 Execution Context
2. T0 決策與目前未完成 task
3. Protected Behavior
4. Implementation Notes

不得預設掃描整個 `.specs` 目錄。定位命令：

```bash
rg -n '^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:|T0' .specs/2026-07-29-10-52_Refactor-config-init-contract
```

## Implementation Notes

- 2026-07-29：完成 requirements、design、tasks 初稿；未修改任何產品碼、測試、README 或 DESIGN。
- 2026-07-29：目前分支為 `refactor/config-init-contract`；工作樹既有未追蹤 `_workspace/` 不屬於本 spec，禁止加入提交。
- 2026-07-29：選擇 additive v1 相容方向，避免修改公開 Config 欄位型別與 Init 簽章。
- 2026-07-29：T0 初稿尚未完成時，所有產品碼任務維持 Pending。
- 2026-07-29：使用者指示建立分支並 run task；確認已在 `refactor/config-init-contract`，T0 採 `ConfigPatch`、`Resolve`、具名 `Instance`、`New`、`Configure`、`ErrAlreadyConfigured` 與 deprecated Init 相容策略。
- 2026-07-29：TDD Red 因新 API 尚不存在而編譯失敗；完成 ConfigPatch、Instance、New 與 Configure 後轉為 Green。
- 2026-07-29：Config 與初始化目標測試在 race detector 下通過，並連續執行 20 次通過。
- 2026-07-29：新增 LIFO resource cleanup 與 `errors.Join` 驗收，確認 rollback 與 Close 共用相同錯誤保留規則。
- 2026-07-29：完整 `go test -race -count=1 ./...`、`go vet ./...`、格式檢查與 `golangci-lint run ./...` 通過，覆蓋率 92.9%。
- 2026-07-29：差異限於 Allowed Changes；`_workspace/` 維持既有未追蹤狀態，未執行 commit、push 或發布。

## 驗證結果摘要

- 新行為驗證：通過；Config 三態、嚴格驗證、defensive copy、Instance cleanup、I/O error、全域失敗重試與重複設定均有測試
- 回歸驗證：通過；完整 race 測試與目標測試連續 20 次通過
- 品質檢查：通過；go vet、gofmt、golangci-lint 為 0 issues
- 測試覆蓋率：92.9%
- 文件一致性：README、DESIGN、godoc 與公開 API 已同步

## 後續改善

- [ ] 另立檔案路徑、symlink、權限與 redaction 安全 spec
- [ ] 另立 encoder 契約與 SQL dead code 清理 spec
- [ ] 另立 Context fields defensive copy spec
- [ ] 另立一般／SplitOutput benchmark 與鎖競爭評估 spec
- [ ] 另立 CI toolchain、gosec 與覆蓋率閘門 spec
