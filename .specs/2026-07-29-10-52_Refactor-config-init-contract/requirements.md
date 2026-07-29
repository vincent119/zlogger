# 需求文件：補強 Config 與初始化契約

## 來源

- Draft: 無
- Type: Refactor
- Owner: 待確認
- Status: Complete

## 文件定位

本 spec 接續 `_workspace/05_review_summary.md` 與前一份 SplitOutput spec 的後續改善項目，處理一般 logger 的設定解析、驗證、初始化失敗、失敗後重試及檔案資源所有權。本 spec 只建立需求、設計與 tasks，不執行產品碼修改。

參考來源：

- 既有文件：`README.md`、`DESIGN.md`、`AGENTS.md`
- 審查結果：`_workspace/04_architecture_review.md`、`_workspace/05_review_summary.md`
- 既有程式碼：`config.go`、`core.go`、`config_test.go`、`core_test.go`
- 前置成果：`.specs/2026-07-29-10-16_BugFix-split-output-lifecycle/`

## 背景

目前 `DefaultConfig().Merge(&Config{Level: "debug"})` 會把 `AddCaller` 與 `ColorEnabled` 從預設 `true` 改為 `false`。原因是 `Config` 使用一般 `bool` 表示設定，無法區分「未提供」與「明確設為 false」。字串與 slice 又採非空才覆寫，造成欄位間合併規則不一致。

`Init` 不回傳錯誤，以 `sync.Once` 包住初始化。一般檔案輸出建立目錄或開檔失敗時會 `panic`；第一次初始化失敗後也無法安全重試。成功開啟的 `*os.File` 只藏在 zap core 中，呼叫端沒有對應的 cleanup 契約。

此外，未知 `Level`、`Format`、`Outputs` 會被靜默降級或忽略。錯誤設定可能在部署後呈現為錯誤級別、錯誤格式或只輸出到 console，而不是在啟動邊界明確失敗。

## 問題陳述

目前設定模型無法可靠表示部分覆寫，初始化又缺少可檢查的錯誤與資源所有權契約，導致呼叫端無法判斷設定是否生效、無法處理 I/O 失敗，也無法完整釋放一般檔案輸出資源。

## 目標

1. 提供可區分「未提供」與明確零值的部分設定模型，尤其是布林欄位。
2. 將部分設定解析成完整 `Config`，且不得修改輸入物件或與輸入共享可變 slice。
3. 在任何 logger 資源建立前驗證 `Level`、`Format`、`Outputs` 與 file output 必要欄位。
4. 提供回傳 error 的非全域 logger 建構入口，不在 library 內因可恢復的設定或 I/O 錯誤 panic。
5. 明確回傳一般檔案輸出的 cleanup，並使 cleanup 可重複及並行呼叫。
6. 提供回傳 error 的全域初始化入口；失敗不得發布半初始化 logger，且修正設定後可以重試。
7. 保持既有 `Config`、`DefaultConfig`、`Merge`、`Init` 與套件級日誌函式的來源碼相容，並文件化舊入口限制與遷移方式。
8. 以 TDD 覆蓋解析、驗證、錯誤、重試、資源關閉與相容行為。

## 非目標

1. 不變更 SplitOutput 的生命週期、分級路由或每日換檔。
2. 不處理 `NewNoEscapeJSONEncoder`、`DisableHTMLEscaping` 或 `sqlProcessingCore`。
3. 不處理 `LogPath`、`FileName` 的路徑逸出、symlink、檔案權限與敏感資訊遮罩；另立安全性 spec。
4. 不重構所有套件級函式為 instance method，也不移除全域 logger。
5. 不允許執行期間動態重建輸出；`SetLevel` 的既有動態調級行為保持不變。
6. 不新增 YAML、TOML、Viper 或其他設定解析依賴；本次只定義資料模型與解析契約。
7. 不修改 Go module、CI、benchmark 或發布流程。
8. 不在本輪文件工作中修改任何 `.go`、README 或 DESIGN 內容。

## 已定決策

- 採新增 API 的相容演進，不直接把既有 `Config` 的 bool 改成 pointer，也不修改 `Init` 簽章。
- 新部分設定模型的布林欄位使用 pointer 表示三態；字串與 slice 同樣必須能區分未提供與明確零值。
- 完整設定只接受文件列出的值：Level 為 `debug`、`info`、`warn`、`error`、`fatal`；Format 為 `console`、`json`；Outputs 為 `console`、`file`。
- Level 與 Format 保持既有大小寫不敏感行為；解析後正規化為小寫。
- 未知或空白 enum、未知 output、重複 output 均回傳驗證錯誤，不再於新入口靜默降級。
- file output 必須在建立資源前完成設定驗證；未提供 `LogPath` 使用既有預設 `./logs`，明確提供空字串則回傳驗證錯誤。
- 公開 API 名稱採 `ConfigPatch`、`Resolve`、`New`、`Configure`。
- `New` 回傳具名 `Instance` 與 error；`Instance` 提供 `Logger`、`Sync`、`Close`，避免呼叫端遺漏 cleanup。
- `Instance.Close` 只負責關閉本次建構所擁有的資源，且可重複及並行呼叫。
- 新全域初始化入口只在完整建構成功後發布 logger；失敗不得消耗一次性初始化狀態，修正設定後可重試。
- 新全域初始化成功後，再次呼叫 `Configure` 回傳可由 `errors.Is` 判斷的 `ErrAlreadyConfigured`，不得替換既有 logger。
- 既有 `Init` 保留簽章並標示 deprecated；對 `ErrAlreadyConfigured` 維持 no-op，其他錯誤維持 legacy panic 行為，新程式必須使用 `Configure`。

## 待確認項目

- 無。T0 已依使用者「run task」指示採用建議方案；若實作發現公開契約無法滿足驗收，必須先更新本 spec。

## 現有行為

- `Config` 一般 bool 無法表示未提供，部分覆寫會意外關閉預設 true。
- `Merge` 直接改動 receiver，並直接指定 `Outputs` slice，存在別名共享。
- `parseLevel` 對未知值回退 info；encoder 對未知 Format 回退 console。
- 未知 Outputs 被忽略；最後沒有有效 core 時自動回退 console。
- `Init` 無 error，建立目錄或開檔失敗時 panic。
- `sync.Once` 讓初始化失敗後無法重試。
- 一般檔案輸出沒有呼叫端可持有的 cleanup。

## 新行為

- 部分設定可明確表示 `false`、空 slice 與未提供，解析結果以 `DefaultConfig` 為基底。
- 解析與驗證回傳新的完整設定，不修改來源，所有輸入與輸出 slice 在邊界複製。
- 新入口的非法設定在任何目錄或檔案建立前失敗，error 包含欄位脈絡且可由既定錯誤分類判斷。
- 非全域建構成功時回傳可直接使用的 `*zap.Logger` 與 cleanup；失敗時不回傳部分 logger，也不遺留已開啟檔案。
- cleanup 關閉該建構所擁有的一般檔案資源；重複及並行 cleanup 安全，並回傳穩定結果。
- 新全域初始化失敗後，套件仍維持先前可用狀態；修正設定後可再次嘗試。
- 新全域初始化成功後，既有套件級 Debug、Info、Warn、Error、Sync 與 SetLevel 繼續工作。
- 舊入口保持可編譯，README 與 DESIGN 提供新舊入口差異及遷移範例。

## 影響範圍

- 使用者：透過 Config 與 `Init` 初始化 zlogger 的呼叫端
- 功能：預設值、部分覆寫、設定驗證、一般 console／file core 建構、全域發布、cleanup
- API：新增公開型別或函式；既有 API 不刪除、不改簽章
- Data / Storage：不變更日誌內容與既有預設檔名；新增可靠關檔時機
- 文件：README、DESIGN、godoc 需同步新入口與相容限制

## 使用情境

- 作為設定檔使用者，我想只指定 `Level` 而保留 `AddCaller=true` 與 `ColorEnabled=true`，避免未填欄位意外改變預設。
- 作為服務啟動流程維護者，我想在錯誤設定或檔案無法建立時取得 error，以便輸出明確原因並決定是否終止啟動。
- 作為 library 使用者，我想持有 logger 的 cleanup，以便優雅關機時同步並關閉一般檔案輸出。
- 作為維護者，我想讓初始化失敗後可以用修正後設定重試，以便測試與嵌入式使用不受 `sync.Once` 鎖死。

## 驗收情境

### 情境：部分設定保留預設 true

- 測試：`TestConfigPatchResolvePreservesDefaults`
- 假設：部分設定只提供 `Level=debug`
- 當：解析為完整設定
- 那麼：Level 為 debug，AddCaller 與 ColorEnabled 仍為 true，其他欄位等於 DefaultConfig

### 情境：部分設定可明確關閉布林選項

- 測試：`TestConfigPatchResolveExplicitFalse`
- 假設：部分設定明確提供 `AddCaller=false`、`ColorEnabled=false`
- 當：解析為完整設定
- 那麼：兩欄位為 false，未提供欄位維持預設

### 情境：解析不共享可變資料

- 測試：`TestConfigPatchResolveCopiesOutputs`
- 假設：部分設定提供 Outputs slice
- 當：完成解析後分別修改來源 slice 與結果 slice
- 那麼：兩者互不影響，DefaultConfig 也未被修改

### 情境：非法設定在 I/O 前失敗

- 測試：`TestConfigValidateRejectsInvalidValues`
- 假設：依序提供未知 Level、未知 Format、未知或重複 Output、無效 file 必要欄位
- 當：呼叫新建構入口
- 那麼：回傳帶欄位脈絡的 validation error，未建立目錄或檔案，也未發布全域 logger

### 情境：一般檔案建構失敗不 panic

- 測試：`TestNewReturnsFileOpenError`
- 假設：使用 `t.TempDir()` 內的一般檔案作為 LogPath 父路徑，形成決定性錯誤
- 當：建立 file output logger
- 那麼：呼叫不 panic，回傳可追溯底層錯誤，沒有可用 logger 或遺留資源

### 情境：cleanup 關閉資源且冪等

- 測試：`TestNewCleanupIsConcurrentAndIdempotent`
- 假設：一般檔案 logger 建構成功
- 當：完成一次寫入與 Sync 後，由多個 goroutine 呼叫 cleanup
- 那麼：所有呼叫完成且結果一致，底層資源只關閉一次，關閉後不再接受寫入，race detector 無報告

### 情境：全域初始化失敗後可重試

- 測試：`TestConfigureCanRetryAfterFailure`
- 假設：第一次使用非法設定或不可建立的 file output，第二次使用有效 console 設定
- 當：依序呼叫新全域初始化入口
- 那麼：第一次回傳 error 且未發布半初始化狀態，第二次成功，套件級 logger 可正常寫入

### 情境：成功後重複全域初始化受控

- 測試：`TestConfigureRejectsSecondSuccess`
- 假設：第一次全域初始化已成功
- 當：再次呼叫新全域初始化入口
- 那麼：依 T0 決策回傳可判斷錯誤，不 panic、不洩漏第二組資源，也不改變已發布 logger

### 情境：既有入口保持來源碼相容

- 測試：`TestLegacyInitCompatibility`
- 假設：既有程式使用 `Init(nil)`、`DefaultConfig`、`Merge` 與套件級日誌函式
- 當：升級至本次版本並編譯執行既有測試
- 那麼：函式簽章維持不變，既有零設定與 SetLevel 行為不被破壞；已知 Merge bool 語意由文件標為 legacy

## 驗收條件

1. T0 五項公開契約決策已記錄於本 spec，且 requirements、design、tasks 一致。
2. 上述九個驗收情境均有決定性測試，不以實際權限、固定絕對路徑或程序全域 goroutine 數量作斷言。
3. `go test -race -count=1 ./...` 通過。
4. 新增目標測試連續 20 次通過。
5. `go vet ./...`、`gofmt -d *.go`、`golangci-lint run ./...` 通過。
6. 新安全入口對可恢復的設定與 I/O 錯誤不 panic。
7. 既有公開 API 簽章不變，未新增外部依賴。
8. README、DESIGN 與 godoc 清楚區分新安全入口、cleanup 與 legacy API。

## 驗證需求

- Unit：Config 部分解析、驗證、slice 複製、錯誤分類、cleanup 冪等
- Integration：一般 file output 實際寫入、Sync、Close；全域初始化失敗後重試
- 回歸：`go test -race -count=1 ./...`
- 穩定性：目標測試 `-count=20`
- 文件：核對 README、DESIGN、godoc 與新公開 API 完全一致

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | 新舊設定模型並存造成使用者混淆 | README 以新入口為主，建立遷移表並標示 legacy 限制 |
| 風險 | 全域狀態使測試互相污染 | 提供 package-private 狀態測試夾具，禁止平行執行會改全域狀態的測試 |
| 風險 | cleanup 與 logger 寫入競爭 | 明確定義 ownership 與並行契約，以 race 測試驗證 |
| 風險 | console Sync 在部分作業系統回傳不支援錯誤 | cleanup 只關閉 owned resources；Sync 錯誤仍由 logger／套件 Sync 契約處理 |
| 風險 | legacy Init 無 error，無法完整轉譯新錯誤契約 | T0 明確決定相容策略，文件不得宣稱 legacy Init 安全處理錯誤 |
| 假設 | 本次版本仍屬 v1 相容演進 | 因既有 module path 無 major suffix，禁止直接破壞公開簽章 |

## 摘要

- 關鍵方向：新增三態部分設定、嚴格驗證、可回傳錯誤與 cleanup 的建構／全域入口
- 相容策略：既有 Config 與 Init 保持可編譯，透過新 API 漸進遷移
- 決策閘門：公開名稱、生命週期承載方式、成功後重複初始化、legacy Init 策略、空白 LogPath 規則
- 下一步：完成 T0 審閱後才可執行 TDD；目前不得修改產品碼
