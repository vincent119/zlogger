# 需求文件：建立檔案輸出安全邊界

## 來源

- Draft: 無
- Type: BugFix
- Owner: 待確認
- Status: Complete

## 文件定位

本 spec 接續 `_workspace/02_security_review.md`、`_workspace/05_review_summary.md` 與已完成的 Config／初始化契約 spec，處理一般 file output 與 SplitOutput 的檔名逸出、安全預設權限、既有目標 symlink 及敏感資訊使用指引。

本輪只建立 requirements、design、tasks，不修改產品碼、測試或公開文件。後續實作必須由使用者另行指示。

參考來源：

- 既有程式碼：`config.go`、`core.go`、`split_output.go`、`fields.go`
- 既有測試：`config_test.go`、`core_test.go`、`split_output_test.go`、`fields_test.go`
- 公開文件：`README.md`、`DESIGN.md`
- 前置規格：`.specs/2026-07-29-10-52_Refactor-config-init-contract/`

## 背景

一般 file output 目前直接以 `filepath.Join(LogPath, FileName)` 組合路徑；SplitOutput 則把 `filePrefix` 與 level、日期組合後再 Join。`FileName` 或 `filePrefix` 若包含 `../`、絕對路徑或路徑分隔符，可能讓輸出逸出呼叫端預期的日誌目錄。

現有 `os.OpenFile` 會跟隨既有 symlink。若預期檔名已被建立為指向目錄外的 symlink，logger 可能附加到基準目錄外的檔案。目錄使用 `0755`、檔案使用 `0644` 建立，也可能讓同機其他帳號讀取含識別資訊、錯誤細節或業務資料的日誌。

專案另提供 `Any`、`Reflect`、SQL 與 error 欄位，但 README 沒有集中說明 token、密碼、Authorization、cookie 與完整個資不得寫入日誌，也沒有不接收原始秘密值的明確遮罩 helper。

## 威脅模型

- `LogPath` 與 SplitOutput 的 `directory` 是呼叫端信任並選定的基準目錄，可為相對或絕對路徑，也可由受信任的部署環境映射至 symlink 目錄。
- `FileName` 與 `filePrefix` 視為不可信任的 leaf name，不得選擇子目錄或逸出基準目錄。
- 本 spec 防止既有最終檔案 symlink 被直接跟隨，但不宣稱在 Go 1.21 下可抵抗攻擊者並行置換檔案所形成的 TOCTOU race。
- 若不可信任者可直接控制 `LogPath` 或 `directory`，呼叫端本身已授權其選擇基準目錄，不屬於本 library 可單獨修復的邊界。

## 問題陳述

目前檔案輸出沒有共享且可驗證的 leaf-name 與開檔安全邊界，預設權限也不符合日誌可能含敏感資料的最小權限原則；使用者同時缺少避免秘密值進入日誌的基本 guardrail。

## 目標

1. `Config.Validate` 在 file output 啟用時拒絕可逸出的 `FileName`。
2. `NewSplitOutput` 在建立目錄或檔案前拒絕可逸出的 `filePrefix`。
3. 一般 file output 與 SplitOutput 共用相同 leaf-name 驗證規則與安全開檔 helper。
4. 拒絕絕對路徑、`.`、`..`、NUL、正斜線、反斜線及 Windows drive-prefix 類型輸入。
5. 空白 `FileName` 保持使用日期檔名；空白 `filePrefix` 保持既有可用行為。
6. 拒絕已存在且最終元件為 symlink 的目標檔案，不修改 symlink 指向內容。
7. 新建目錄預設要求 `0700`，新建日誌檔預設要求 `0600`；不得主動 chmod 既有目錄或檔案。
8. 提供可由 `errors.Is` 判斷的 `ErrUnsafeLogPath`，同時保留 `ErrInvalidConfig` 的設定錯誤分類。
9. 新增 `Redacted(key string) Field`，固定輸出 `[REDACTED]` 且不接收秘密值。
10. README 明確列出禁止記錄的秘密與個資類別、結構化欄位白名單原則及 `Redacted` 用法。
11. 保持安全檔名、既有日期格式、分級路由、換檔生命週期與公開函式簽章相容。

## 非目標

1. 不自動掃描訊息或欄位名稱，不以黑名單猜測秘密值。
2. 不修改或遮罩既有 `String`、`Any`、`Reflect`、`Err` 行為。
3. 不允許呼叫端在本次新增自訂 DirPerm／FilePerm；需要時另立 options spec。
4. 不主動 chmod 既有路徑，避免改變其他程序共享資源的權限。
5. 不拒絕受信任基準目錄本身為 symlink。
6. 不宣稱抵抗 symlink 並行置換 race；原子 containment 需先將 module／CI 基線升至 Go 1.24+ 並採 `os.Root`，另立工具鏈相依 spec。
7. 不更新 `go.mod`、CI、GitHub Actions、依賴或 Go 最低版本。
8. 不處理 SQL core、encoder、Context slice 或效能問題。

## 已定決策

- 新增 exported sentinel `ErrUnsafeLogPath`。
- leaf-name 規則同時套用一般 `FileName` 與 SplitOutput `filePrefix`。
- `FileName=""` 與 `filePrefix=""` 為既有允許行為；其他值不得包含路徑語意。
- 基準目錄由呼叫端信任；安全邊界限制 leaf name，不阻止呼叫端選擇絕對 LogPath。
- 既有最終目標為 symlink 時直接回傳 `ErrUnsafeLogPath`；不跟隨、不移除、不覆寫。
- 新目錄要求 `0700`、新檔要求 `0600`；實際權限仍可能被 umask 進一步收緊。
- 既有檔案與目錄權限保持不變。
- `Redacted` 不接受 value，避免 helper 本身接觸秘密內容。
- 不新增外部依賴，不提高 Go 最低版本。

## 待確認項目

- 無。若實作發現標準庫無法在 Protected Behavior 下滿足驗收，必須先更新 spec，不得降低 symlink 或 containment 測試要求。

## 現有行為

- `FileName="../outside.log"` 可能透過 Join 逸出 LogPath。
- `filePrefix="../outside"` 可能讓三個分級檔寫到 directory 之外。
- 既有最終目標 symlink 會被 `os.OpenFile` 跟隨。
- 新目錄使用 `0755`，新檔使用 `0644`。
- README 未集中說明秘密與完整個資不得寫入日誌。
- 沒有明確的 redacted field helper。

## 新行為

- 不安全 FileName 在 Config 驗證階段失敗，任何目錄或檔案都尚未建立。
- 不安全 filePrefix 在 NewSplitOutput 建立目錄前失敗，且不啟動 worker。
- 安全 leaf name 只能在呼叫端選定的基準目錄直接建立檔案。
- 若最終目標已是 symlink，安全開檔 helper 回傳包裝 `ErrUnsafeLogPath` 的錯誤。
- 新建立的目錄不授予 group／other 權限；新建立的檔案不授予 group／other 讀寫權限。
- 已存在的一般檔案仍以 append 模式寫入，且原權限不變。
- `Redacted("authorization")` 只記錄固定 `[REDACTED]` 字串。

## 影響範圍

- 使用者：使用 Config file output、NewSplitOutput、GetSplitCore 與結構化欄位的呼叫端
- API：新增 `ErrUnsafeLogPath` 與 `Redacted`；既有簽章不變
- Storage：新建目錄／檔案權限收緊；不安全檔名與 symlink 由成功改為錯誤
- 文件：README、DESIGN、godoc 增加安全邊界、剩餘限制與遷移說明
- 相容性：安全檔名與既有檔案正常；依賴跨目錄 FileName／prefix 或 group-readable 新檔的呼叫端需調整

## 使用情境

- 作為服務維運者，我想確保可設定的檔名不能逸出指定日誌目錄，以免設定錯誤寫到其他路徑。
- 作為同機多使用者環境管理者，我想讓新日誌預設只有程序帳號可讀寫，降低資訊暴露。
- 作為 library 使用者，我想在遇到不安全路徑時以 `errors.Is` 分類處理，而不是解析錯誤文字。
- 作為開發者，我想明確標記不能記錄的欄位，且不把秘密值傳入遮罩 helper。

## 驗收情境

### 情境：Config 拒絕不安全 FileName

- 場景：file output 使用帶路徑語意的 FileName
- 測試：`TestConfigValidateRejectsUnsafeFileName`
- 假設：Outputs 包含 file，FileName 依序為 `../outside.log`、絕對路徑、`sub/file.log`、`sub\\file.log`、`.`、`..`、含 NUL 或 drive prefix
- 當：呼叫 `Validate`、`Resolve` 或 `New`
- 那麼：回傳同時可判斷為 `ErrInvalidConfig` 與 `ErrUnsafeLogPath` 的錯誤，且沒有建立任何路徑

### 情境：SplitOutput 拒絕不安全 prefix

- 場景：分級輸出使用帶路徑語意的 filePrefix
- 測試：`TestNewSplitOutputRejectsUnsafePrefix`
- 假設：directory 尚不存在，prefix 使用 traversal、絕對路徑、正反斜線或 drive prefix
- 當：呼叫 `NewSplitOutput`
- 那麼：回傳可判斷為 `ErrUnsafeLogPath` 的錯誤，directory 未建立，worker 未啟動

### 情境：安全名稱保持在基準目錄

- 場景：一般與分級輸出使用合法 leaf name
- 測試：`TestFileOutputsStayWithinBaseDirectory`
- 假設：使用 `t.TempDir()` 下的新 base directory
- 當：建立 logger、寫入並完成 Sync／Close
- 那麼：所有檔案只存在 base directory 直接子層，外部同名檔不存在，既有日期與 level 檔名格式不變

### 情境：拒絕既有最終 symlink

- 場景：預期日誌檔已是指向 base 外部檔案的 symlink
- 測試：`TestFileOutputsRejectExistingSymlink`
- 假設：平台允許建立 symlink，外部檔含固定內容
- 當：一般 file output 或 SplitOutput 嘗試開啟該名稱
- 那麼：回傳 `ErrUnsafeLogPath`，外部內容不變，失敗建構不遺留其他已開檔案或 worker

### 情境：新路徑使用最小權限

- 場景：建立新的 logger 目錄與檔案
- 測試：`TestFileOutputsUsePrivatePermissions`
- 假設：POSIX 平台、路徑尚不存在
- 當：一般 file output 與 SplitOutput 建立資源
- 那麼：目錄 group／other bits 為 0，檔案 group／other bits 為 0；測試允許 umask 進一步收緊

### 情境：既有路徑權限不被改寫

- 場景：呼叫端預先建立可接受的既有檔案
- 測試：`TestFileOutputPreservesExistingPermissions`
- 假設：POSIX 平台、既有檔案 mode 為 `0640`
- 當：logger 以 append 模式開啟、寫入並關閉
- 那麼：內容追加成功，mode 仍為 `0640`

### 情境：Redacted 不接收秘密值

- 場景：呼叫端標記敏感欄位
- 測試：`TestRedactedField`
- 假設：只提供欄位 key
- 當：將 `Redacted("authorization")` 寫入 JSON logger
- 那麼：輸出包含固定 `[REDACTED]`，API 不接受原始秘密 value

### 情境：既有安全行為不回歸

- 場景：使用安全 FileName 與 prefix 執行一般及分級輸出
- 測試：既有 `TestNewCleanupIsConcurrentAndIdempotent`、`TestGetSplitCoreRoutesLevels`、`TestSplitOutputCloseStopsRotation`
- 假設：名稱不含路徑語意
- 當：建立、寫入、換檔、同步與關閉
- 那麼：內容、路由、檔名、cleanup 與 worker 生命週期保持既有契約

## 驗收條件

1. 上述八個驗收情境均有決定性測試。
2. traversal 驗收同時覆蓋 Unix 與 Windows 形式分隔符，不依賴執行平台才判斷。
3. symlink 測試在不支援建立 symlink 的平台明確 skip，不得誤判通過。
4. permission 測試檢查 group／other bits，而不是假設 umask 後必為精確 mode。
5. `go test -race -count=1 ./...` 通過。
6. 安全目標測試連續 20 次通過。
7. `go vet ./...`、`gofmt -d *.go`、`golangci-lint run ./...` 通過。
8. 未修改既有公開函式簽章、go.mod、go.sum 或 CI。
9. README 與 DESIGN 明確記錄 trusted base、leaf-name 規則、權限改變、symlink 剩餘風險與敏感資訊指引。

## 驗證需求

- Unit：leaf-name table、Config error chain、Redacted field
- Integration：一般／分級輸出 containment、symlink、mode、既有檔 append
- 回歸：`go test -race -count=1 ./...`
- 穩定性：安全目標測試 `-count=20`
- 文件：核對 README、DESIGN、godoc 與 threat model 一致

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | Lstat 與 OpenFile 間存在 TOCTOU | 明確揭露剩餘風險；Go 基線升版後改採 os.Root |
| 風險 | 收緊權限影響依賴 group-readable 新檔的部署 | PR 與 README 列為行為變更；既有檔不 chmod |
| 風險 | 拒絕反斜線會改變 Unix 上原可用檔名 | 採跨平台一致安全規則，換取設定可攜性 |
| 風險 | Redacted 被誤認為自動遮罩 | 文件明示只產生固定欄位，其他 API 不會自動偵測秘密 |
| 假設 | LogPath／directory 是可信任 base | 寫入 threat model；若不成立由呼叫端限制設定來源 |

## 摘要

- 關鍵決策：可信任 base、不可信任 leaf；拒絕 traversal 與既有最終 symlink；新路徑採 0700／0600
- 敏感資訊：新增不接收 value 的 Redacted helper，並補 README guardrail
- 剩餘風險：Go 1.21 標準庫無法提供本 spec 所需的原子 containment，TOCTOU 留待工具鏈升版後處理
- 下一步：審閱 design.md 與 tasks.md；未經使用者指示不得開始實作
