# 需求文件：通用分級 Sink

Status: Complete

## 文件定位

本 spec 接續既有 `SplitOutput` 分級路由與生命週期實作，以及 README、DESIGN 對
timberjack 外部整合的既有決策。目標是在不引入 timberjack dependency、不改變每日
換檔與安全開檔契約的前提下，提供公開且通用的分級 sink 注入 API。

本 spec 不重寫已完成的 `SplitOutput` lifecycle、`os.Root` containment、檔案權限、
Config 初始化、encoder compatibility、Context 或效能基線。

## 背景

目前 `GetSplitCore` 會建立三個檔案輸出，並把 DEBUG／INFO、WARN、ERROR 以上分別
路由至 info、warn、error。呼叫端若要使用 timberjack、記憶體 sink、網路 sink 或
其他 `zapcore.WriteSyncer`，必須自行重複建立三個 `zapcore.Core`、三組 level enabler
及 `zapcore.NewTee`。

現有 `splitFilePermissionOpener` 與 `splitFileSet` 均為 package-private，且 opener 同時
承擔檔案命名、權限及每日換檔生命週期，不適合作為公開擴充點。直接公開 opener 或將
timberjack 寫入 `Config`，會混淆分級路由、資源所有權與 rotation 責任。

## 現有行為

1. `NewSplitOutput`／`NewSplitOutputWithOptions` 建立三個每日檔案並啟動換檔 worker。
2. `GetSplitCore`／`GetSplitCoreWithOptions` 建立 JSON encoder、三個 level core 與 cleanup。
3. DEBUG、INFO 寫入 info；WARN 寫入 warn；ERROR、DPANIC、PANIC、FATAL 寫入 error。
4. 現有 cleanup 擁有並關閉 `SplitOutput` 建立的檔案與 worker。
5. 外部 sink 沒有受支援的分級 core 建構 API。

## 目標

1. 新增公開 `SplitSinks`，讓呼叫端提供 info、warn、error 三個
   `zapcore.WriteSyncer`。
2. 新增公開 `NewSplitCore`，統一建立無重複寫入的三路分級 core。
3. 保持既有 `SplitOutput`、`GetSplitCore` 與公開函式簽章向下相容。
4. 讓現有檔案型 `GetSplitCoreWithOptions` 與新 API 共用同一份 level routing 建構邏輯。
5. 明確規定注入 sink 的 ownership、`Sync` 與錯誤行為。
6. 提供可編譯範例，證明一般自訂 sink 可直接接入，並說明 timberjack 的接法。

## 非目標

1. 不新增 timberjack 或其他 log rotation dependency。
2. 不提供 timberjack Adapter、rotation 設定型別或三個 timberjack 實例的建構器。
3. 不讓 `NewSplitCore` 建立、關閉或替換呼叫端提供的 sink。
4. 不修改每日換檔時間、檔名格式、檔案權限、`os.Root` 或 symlink 防護。
5. 不改變 Config／ConfigPatch schema、全域 logger 或一般 file output。
6. 不提供動態 sink Factory、runtime sink replacement、buffer 或非同步寫入。
7. 不承諾非標準 zap level 的路由行為。

## 公開契約需求

### `SplitSinks`

公開型別必須包含三個具名欄位：

```go
type SplitSinks struct {
	Info  zapcore.WriteSyncer
	Warn  zapcore.WriteSyncer
	Error zapcore.WriteSyncer
}
```

欄位語意：

| 欄位 | 接收 level |
|------|------------|
| `Info` | DEBUG、INFO |
| `Warn` | WARN |
| `Error` | ERROR、DPANIC、PANIC、FATAL |

### `NewSplitCore`

公開建構式採下列契約：

```go
func NewSplitCore(
	encoder zapcore.Encoder,
	sinks SplitSinks,
) (zapcore.Core, error)
```

1. `encoder` 或任何必要 sink 為 nil 時，回傳可由 `errors.Is` 判斷為
   `ErrInvalidSplitCore` 的錯誤，不回傳部分 core。
2. 建構成功後，每個標準 level 只寫入對應 sink 一次。
3. 三個 core 使用各自的 encoder clone，避免共享可變 encoder 狀態。
4. `NewSplitCore` 不取得 encoder 或 sink 的關閉所有權。
5. `core.Sync()` 會委派至三個配置欄位；呼叫端重用同一 sink 實例於多個欄位時，
   該 sink 可能收到多次 `Sync`，文件必須說明應提供三個獨立 sink。
6. `NewSplitCore` 不回傳 cleanup；呼叫端自行管理外部 sink 的 `Close`。

### 錯誤契約

新增公開 sentinel：

```go
var ErrInvalidSplitCore = errors.New("分級 core 設定無效")
```

錯誤訊息必須指出缺少的是 encoder、info、warn 或 error，不得包含 sink 內部資料。

## 使用情境

1. Library 使用者建立三個 timberjack logger，將其作為 `SplitSinks` 注入，不需要自行
   撰寫 level enabler 或 `zapcore.NewTee`。
2. 測試程式以三個記憶體 sink 驗證不同等級輸出。
3. 使用者以自訂網路或檔案 `zapcore.WriteSyncer` 建立分級 logger，並自行管理連線及
   `Close`。
4. 既有使用者繼續呼叫 `GetSplitCore`，得到相同檔名、路由與 cleanup 行為。

## 驗收情境

### 場景一：標準 level 分級且不重複

- 測試：`TestNewSplitCoreRoutesLevels`
- 假設：提供三個獨立記憶體 sink 與可用 encoder
- 當：依序寫入 DEBUG、INFO、WARN、ERROR、DPANIC、PANIC、FATAL
- 那麼：DEBUG／INFO 只出現在 info，WARN 只出現在 warn，其他四級只出現在 error

### 場景二：缺少必要輸入立即失敗

- 測試：`TestNewSplitCoreRejectsInvalidInputs`
- 假設：encoder 或三個 sink 其中之一為 nil
- 當：呼叫 `NewSplitCore`
- 那麼：回傳 nil core，錯誤可由 `errors.Is(err, ErrInvalidSplitCore)` 判斷，且未寫入 sink

### 場景三：Sync 委派

- 測試：`TestNewSplitCoreSyncsSinks`
- 假設：三個獨立 sink 可記錄 `Sync` 次數與回傳錯誤
- 當：呼叫 `core.Sync()`
- 那麼：每個配置欄位各收到一次 `Sync`；錯誤依 zap core 既有語意回傳

### 場景四：外部資源所有權不轉移

- 測試：`TestNewSplitCoreDoesNotCloseSinks`
- 假設：三個 sink 同時實作 `Close`
- 當：建立及使用 core，並呼叫 `core.Sync()`
- 那麼：`zlogger` 不呼叫任何 sink 的 `Close`；呼叫端仍可自行關閉

### 場景五：既有 SplitOutput 完全相容

- 測試：`TestGetSplitCoreRoutesLevels`、`TestSplitOutputCloseStopsRotation`、
  `TestGetSplitCoreWithOptionsUsesConfiguredPermissions`
- 假設：使用既有公開 API 建立每日分級輸出
- 當：寫入、同步、換檔並執行 cleanup
- 那麼：路由、檔名、權限、安全邊界與關閉生命週期均維持既有行為

### 場景六：公開範例可編譯並呈現 ownership

- 測試：`ExampleNewSplitCore`
- 假設：使用標準記憶體 sink 或無外部 dependency 的可控 sink
- 當：執行 example test
- 那麼：輸出顯示三個 level 分別進入正確 sink，範例由呼叫端管理 sink 生命週期

## 驗收條件

1. 新 API 與 sentinel 具繁體中文 Go doc，`go doc` 可辨識 routing 與 ownership。
2. 新舊路徑共用 level enabler／core 組裝 helper，不保留兩份可漂移的 routing 規則。
3. 新增測試涵蓋七個標準 level、nil 輸入、Sync 與不接管 Close。
4. 既有 SplitOutput lifecycle、安全、權限、routing 與 race 測試全部通過。
5. README 提供通用 sink 用法及 timberjack 整合片段，但 `go.mod`／`go.sum` 不變。
6. DESIGN 記錄路由與 rotation 分層、ownership 決策及不提供 Factory 的理由。
7. 覆蓋率不得低於現有 gate，`go test -race`、`go vet`、lint 與 `git diff --check` 通過。

## 驗證需求

```bash
go test -count=1 -run 'TestNewSplitCore|ExampleNewSplitCore' ./...
go test -race -count=1 -run 'Test(NewSplitCore|GetSplitCore|SplitOutput)' ./...
go test -count=20 -run 'Test(NewSplitCoreRoutesLevels|GetSplitCoreRoutesLevels)' ./...
make verify
git diff --check
```

另外確認：

```bash
git diff -- go.mod go.sum
rg -n 'timberjack' go.mod go.sum
```

兩項 dependency 檢查必須沒有新增 timberjack。

## 影響範圍

| 範圍 | 影響 |
|------|------|
| `split_output.go` | 新增公開型別、錯誤、建構式與共用 core helper |
| `split_output_test.go` | 新增 routing、validation、Sync、ownership 與回歸測試 |
| `example_test.go` 或專用 example 檔 | 新增可編譯公開範例 |
| `README.md` | 新增自訂 sink 與 timberjack 分級整合說明 |
| `DESIGN.md` | 記錄分層、所有權及相容性決策 |
| `go.mod`／`go.sum` | 不得變更 |
