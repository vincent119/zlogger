# README 使用契約與樣本正確性任務

Status: Complete

## Execution Context

### 意圖

修正 README 使用樣本、設定檔說明與實際公開 API 的落差，並讓 `color_enabled` 的
實作符合「僅 console 有顏色」的既有文件契約。

### 非目標

1. 不新增 Gin middleware package。
2. 不加入 timberjack production dependency 或 adapter。
3. 不重構 global logger state。
4. 不改變 `Configure`、`Init`、`SetLevel` 的函式簽章與 legacy 契約。
5. 不建立 release tag 或修改 CI/CD。
6. 不新增任何設定檔 loader 或設定框架依賴。

### 已定決策

1. JSON 永遠不輸出 ANSI 彩色 level。
2. console 僅在 `color_enabled=true` 時保留彩色 level。
3. timberjack 單檔樣本直接使用建立出的 `*zap.Logger`，不暗示它會設定 zlogger global。
4. Gin 樣本移除未使用設定欄位，不為其補造未需求的功能。
5. 生命週期與錯誤行為以目前測試及原始碼為準，先補文件，不擴張 API。
6. zlogger 僅接收 `ConfigPatch` 或 `Config`；檔案讀取、反序列化、ENV 優先級與
   未知 key 檢查由呼叫端負責。
7. README 使用單一 Mermaid 高階架構圖，標準 logger 與分級輸出使用不同區塊。
8. 繁體中文版是主要契約；英文版必須在同一 task 同步相同結構與技術內容。

### 邊界

允許修改 `core.go`、對應測試、`README.md` 及必要的 Go example 測試。若需要修改其他
production 檔案，必須先更新本文件的 task Boundary 並說明原因。

### 關鍵檔案

1. `README.md`
2. `core.go`
3. `core_test.go`
4. `context.go`
5. `split_output.go`
6. `split_output_example_test.go`

### 完成條件

1. requirements 的 AC-1 至 AC-9 通過。
2. 本文件所有 task 完成並標記完成。
3. race test、vet、文件檢查及 `git diff --check` 通過。

## Protected Behavior

1. DEBUG、INFO 寫入 info sink；WARN 寫入 warn sink；ERROR 以上寫入 error sink。
2. `SplitOutput` 每日換檔、關閉冪等、goroutine 收斂及關閉後錯誤行為不變。
3. 預設目錄與檔案權限維持 `0700`／`0600`。
4. `Configure` 第二次成功設定仍回傳 `ErrAlreadyConfigured`。
5. `SetLevel` 未知字串仍回退 `info`。
6. `NewSplitCore` 不取得外部 sink ownership。
7. context fields 的 defensive copy 行為不變。
8. `ConfigPatch.Resolve` 的預設值、正規化、複製與驗證行為不變。

## 實作任務

### T1：新增 encoder 顏色契約測試

- [x] 建立 `console/json × color true/false` 的 table-driven test。
- [x] 驗證 JSON level 不含 ANSI escape sequence。
- [x] 驗證 console 彩色與停用顏色行為。

Boundary:

- Allowed Changes：`core_test.go`，或新增單一對應的 `*_test.go`。
- Forbidden：production code、README、外部依賴。

Depends: 無。

Context: 先以失敗測試固定 AC-1 至 AC-3，再修改 encoder 選擇條件。

Verify: `go test ./... -run 'TestBuildEncoderConfig|Test.*Color'`

### T2：修正 JSON 顏色行為

- [x] `buildEncoderConfig` 僅在 console 且啟用顏色時使用彩色 level encoder。
- [x] 確認 console 的既有顏色輸出未改變。

Boundary:

- Allowed Changes：`core.go`。
- Forbidden：設定 schema、公開 API 簽章、logger 組裝與輸出路由。

Depends: T1。

Context: 使用 `cfg.Format` 與 `cfg.ColorEnabled` 的聯合條件；不新增設定欄位。

Verify: `go test ./... -run 'TestBuildEncoderConfig|Test.*Color'`

### T3：修正 Gin 使用樣本

- [x] 將 `middleware.skipMiddlewareLog` 改為 `middleware.SkipMiddlewareLog`。
- [x] 移除 `Zconfig` 中未生效的 `TimeFormat`、`UTC`、`DefaultLevel`。
- [x] 移除因欄位刪除而不再需要的 import 或預設值。
- [x] 修正樣本縮排，確保符號與 import 一致。

Boundary:

- Allowed Changes：`README.md` 與 `README.en.md` 的 Gin middleware 與 handler 樣本。
- Forbidden：建立正式 middleware package、改動 production Go code、增加 Gin 依賴。

Depends: 無。

Context: README 樣本只保留實際有使用的 `SkipPaths`、`Context`、`Category`。

Verify: `rg -n 'skipMiddlewareLog|TimeFormat\s+|DefaultLevel\s+|UTC\s+bool' README.md README.en.md`
不得在 `Zconfig` 留下失效欄位，並以暫存 module 編譯檢查樣本。

### T4：修正 timberjack 單檔與分級樣本

- [x] 移除單檔樣本未使用的 `zlogger` import。
- [x] 讓單檔樣本直接使用建立出的 logger，不錯誤宣稱已設定 zlogger global。
- [x] 清楚區分 `zap.ReplaceGlobals` 與 zlogger package-level logger。
- [x] 補充 `ErrInvalidSplitCore` 的判斷入口。
- [x] 保留三個外部 sink 的 `Sync`、`Close` ownership 說明。

Boundary:

- Allowed Changes：`README.md`、`README.en.md` 的 timberjack 與自訂分級 sink
  章節；必要時調整既有
  `split_output_example_test.go` 以維持相同核心模式。
- Forbidden：將 timberjack 加入 `go.mod`、新增 adapter、修改 `NewSplitCore` 實作。

Depends: 無。

Context: 單檔樣本是獨立 zap logger；分級樣本使用 zlogger 的 `NewSplitCore`，兩者不應
混為同一種 global 初始化方式。

Verify: `go test ./... -run Example`，另以暫時 module 或等效方式編譯 README 的
timberjack 樣本，不提交外部依賴變更。

### T5：補齊初始化、生命週期與錯誤契約

- [x] 比較 `Configure`、`ConfigureWithOptions`、`Init`、`New`／`NewWithOptions`。
- [x] 記錄 `ErrAlreadyConfigured` 與 cleanup 不重設 Configure 資格。
- [x] 記錄 `Init` 初始化失敗可能 panic，並推薦新程式使用 `Configure`。
- [x] 記錄 `Instance.Close`、`Instance.Sync` 關閉後行為。
- [x] 記錄 `SetLevel` 未知值回退 `info`。

Boundary:

- Allowed Changes：`README.md`、`README.en.md` 的初始化、設定與生命週期相關章節。
- Forbidden：修改上述 API 實作、改變 legacy 行為。

Depends: 無。

Context: 文件內容須逐項對照 `core.go` 與既有測試，不推導程式碼沒有保證的行為。

Verify: `rg -n 'ConfigureWithOptions|ErrAlreadyConfigured|os.ErrClosed|SetLevel|panic' README.md README.en.md`

### T6：補充公開 API 導覽

- [x] 加入初始化、instance、context、分級輸出的精簡 API 索引。
- [x] 納入 `FromContext`、`WithOperation`、`WithComponent`。
- [x] 納入 `NewSplitCore`、`SplitSinks`、`ErrInvalidSplitCore`。
- [x] 避免重複大段 GoDoc 或擴張成完整 API reference。

Boundary:

- Allowed Changes：`README.md`、`README.en.md`。
- Forbidden：新增 API、全面重排 README。

Depends: T4、T5。

Context: API 索引應指向既有章節或附一行用途，保持 README 可讀性。

Verify: `rg -n 'FromContext|WithOperation|WithComponent|NewSplitCore|SplitSinks|ErrInvalidSplitCore' README.md README.en.md`

### T7：明確化設定檔與設定值契約

- [x] 明示 zlogger 不負責讀取 YAML、JSON、TOML 或 ENV。
- [x] 列出九個設定 key、型別、預設值、合法值與條件式必填規則。
- [x] 說明 `ConfigPatch` pointer 的未提供與明確零值語意。
- [x] 說明 level、format、outputs 的小寫正規化與 Outputs defensive copy。
- [x] 說明未知 key 必須由 decoder 嚴格模式拒絕，`Validate` 只驗證已映射的值。
- [x] 區分 decoder／I/O 錯誤、`ErrInvalidConfig`、`ErrUnsafeLogPath`。
- [x] 說明檔案 permissions 使用 functional options，不是設定檔欄位。
- [x] 若保留具體 loader 範例，補齊可執行解析流程及額外依賴說明。

Boundary:

- Allowed Changes：`README.md`、`README.en.md` 的設定檔載入、設定選項與直接相關的
  初始化或檔案安全交叉連結。
- Forbidden：修改 `Config`／`ConfigPatch` 實作、加入 loader、修改 `go.mod`、新增
  filesystem permission 設定欄位。

Depends: T2、T5。

Context: 設定說明逐項以 `config.go`、`file_options.go` 與現有測試為準。若 decoder
忽略未知 key，zlogger 收到 struct 後無法還原該資訊，因此不得宣稱 `Validate` 會拒絕
設定檔的未知 key。

Verify: 檢查兩份 README 均包含 `ConfigPatch`、`ErrInvalidConfig`、`ErrUnsafeLogPath`、
`mapstructure`、未知 key、預設值、`log_path`、`file_name`、`color_enabled`。

### T8：加入 README 高階架構圖

- [x] 在兩份 README 的專案簡介與安裝章節之間加入對應語言的架構概覽。
- [x] 使用 Mermaid `flowchart`，不新增 PNG 或 SVG。
- [x] 呈現外部設定來源由呼叫端解析後進入 `ConfigPatch`。
- [x] 分開呈現 `Configure` global 路徑與 `New` instance 路徑。
- [x] 分開呈現標準 console／file、`SplitOutput` 每日三檔與外部 `SplitSinks`。
- [x] 在圖後補充 rotation 與 sink ownership 邊界。

Boundary:

- Allowed Changes：`README.md`、`README.en.md` 的專案簡介後新增架構章節。
- Forbidden：新增圖片資產、修改 production code、把內部私有函式當作公開架構節點。

Depends: T4、T5、T7。

Context: 圖必須反映公開契約；`Configure`／`New` 不得連到 `SplitOutput`，外部 decoder
不得畫在 zlogger package 內，`NewSplitCore` 不得畫成擁有外部 sinks。

Verify: 檢查兩份 README 的 Mermaid 語法可渲染、node／edge 關係一致，並人工核對
圖中每條連線與 `core.go`、`split_output.go` 的公開契約。

### T9：同步版本可用性說明

- [x] 確認 README 所述 API 是否已包含於最新發布 tag。
- [x] 若尚未發布，清楚標示 main 與已發布版本差異或發布前置條件。
- [x] 不在本 task 建立 tag 或 GitHub Release。

Boundary:

- Allowed Changes：`README.md`、`README.en.md` 的安裝或版本說明。
- Forbidden：tag、release、workflow、`go.mod` module path。

Depends: T6、T7、T8。

Context: 不允許讓 `go get github.com/vincent119/zlogger` 的使用者誤以為舊 tag 已包含新 API。

Verify: 比對 `git tag`、最新 tag 的公開符號及 README 安裝說明。

### T10：同步中英文 README 結構

- [x] 以繁體中文版作為主要契約，補齊英文版缺少的章節。
- [x] 兩份文件的 heading 層級與順序一致。
- [x] Go、YAML、JSON、shell code blocks 的技術內容一致。
- [x] 設定 key、預設值、合法值、API、錯誤值與命令一致。
- [x] 兩份 Mermaid 圖的 node、edge 與 subgraph 關係一致。
- [x] 兩份文件頂端提供正確的語言切換連結。
- [x] 不使用跨語言 heading anchor 連結。

Boundary:

- Allowed Changes：`README.md`、`README.en.md`，限雙語結構與內容同步。
- Forbidden：production code、測試行為、圖片資產、建立 `docs/` 目錄、逐字機械翻譯
  導致技術語意改變。

Depends: T3 至 T9。

Context: 英文版目前明顯短於中文版，不能只補新增段落；應依最終中文版逐章補齊，
但技術契約以原始碼及測試為最高依據。

Verify: 比對兩份 README 的 heading tree、fenced code blocks、API identifiers、設定 key、
錯誤值、Mermaid edges 與相對連結。

## 驗證任務

### T11：完整品質驗證

- [x] 執行目標測試。
- [x] 執行完整 race test。
- [x] 執行 vet。
- [x] 檢查雙語 README 樣本、設定表、架構圖與 API 名稱。
- [x] 檢查中英文 heading、code blocks、技術 token 與連結一致性。
- [x] 檢查 diff 範圍與空白錯誤。

Boundary:

- Allowed Changes：僅修正 T1 至 T10 邊界內發現的問題；本 task 不新增功能。
- Forbidden：未納入 spec 的重構、依賴升級、CI/CD 或 release 變更。

Depends: T1 至 T10。

Context: 若驗證需要超出既定 Boundary 的修正，先更新本文件並說明原因。

Verify:

```bash
go test -race -count=1 ./...
go vet ./...
git diff --stat
git diff --check
```

## 品質檢查清單

- [x] README 樣本沒有不存在的 symbol 或未使用 import。
- [x] JSON level 不含 ANSI escape sequence。
- [x] console 彩色行為維持相容。
- [x] 全域與 instance logger 的說明沒有混用。
- [x] resource ownership 與 cleanup 順序清楚。
- [x] legacy 行為均明確標記，未暗示不存在的 error handling。
- [x] 九個設定欄位、預設值、合法值與條件式驗證皆與 `config.go` 一致。
- [x] 明確區分外部 decoder、`ConfigPatch.Resolve` 與 `Config.Validate` 的責任。
- [x] 未宣稱 zlogger 自帶設定檔或環境變數 loader。
- [x] Mermaid 架構圖可渲染，且沒有錯誤表達 global、split 或 ownership 契約。
- [x] 未新增圖片資產。
- [x] 中英文 heading 結構、code blocks、設定表、API 與錯誤值已完成對照。
- [x] 未新增 Gin 或 timberjack production dependency。
- [x] 未新增設定框架 production dependency。
- [x] 未修改 Protected Behavior。
- [x] `go test -race -count=1 ./...` 通過。
- [x] `go vet ./...` 通過。
- [x] `git diff --check` 通過。

## Implementation Notes

2026-07-31：已建立 `bugfix/readme-usage-contract-accuracy` 分支並開始執行 T1。

2026-07-31：T1 測試先確認 JSON level 含 ANSI 的既有問題；T2 將彩色 encoder 限制為
`format=console && color_enabled=true`，目標測試通過。

2026-07-31：T3 至 T10 完成雙語 README 重整。兩份文件均為 12 個 H2、9 個 H3、
22 組 code fences；Gin、timberjack v1.4.5 與嚴格 JSON loader 樣本已在暫存 module
編譯通過。

2026-07-31：T11 完成。`make verify` 使用專案固定的 golangci-lint v2.12.2 執行，
結果為 0 issues；race test、vet、benchmark 通過，總覆蓋率 92.7%。雙語 README
另通過忽略 code fence Tab 與表格長行規則後的 markdownlint 結構檢查。

實作時每完成一項 task，應更新 checkbox；全部驗證通過後才將 `Status` 改為
`Complete`。
