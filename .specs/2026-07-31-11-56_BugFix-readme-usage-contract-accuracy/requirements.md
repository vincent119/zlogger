# README 使用契約與樣本正確性需求

Status: Complete

## 文件定位

本 spec 接續目前 `main` 已完成的設定初始化、檔案輸出安全、可設定檔案權限、
`SplitOutput` 生命週期及通用 `SplitSinks` 功能，修正 `README.md` 與現有公開 API、
實際執行行為不一致的部分。

本 spec 不重寫上述模組，也不改變既有分級路由、檔案權限、路徑防護、context
欄位複製或 rotation ownership 契約。

## 背景

目前中文版 README 大致涵蓋現有功能，但使用者直接複製部分樣本時會遇到編譯錯誤，
或得到與文字說明不同的 logger 行為：

1. Gin handler 使用小寫 `skipMiddlewareLog`，但實際匯出函式為
   `SkipMiddlewareLog`。
2. Gin middleware 的 `TimeFormat`、`UTC`、`DefaultLevel` 欄位有宣告與預設值，
   但樣本流程未使用這些欄位。
3. timberjack 單檔樣本匯入但未使用 `zlogger`，且錯誤暗示
   `zap.ReplaceGlobals` 會讓 `zlogger.Info` 等 package-level 函式改用該 logger。
4. README 宣告 `color_enabled` 僅影響 console，但目前 encoder 在 JSON 格式也會使用
   `CapitalColorLevelEncoder`，使 JSON level 可能包含 ANSI 控制碼。
5. README 尚未清楚描述 `Configure` 的一次性設定契約、`Init` 的 panic 風險、
   `SetLevel` 的未知值回退行為及外部 sink 的 ownership。
6. 部分已公開 API 與錯誤值缺少最小可發現的索引或樣本。
7. 「從設定檔載入」只留下實際解析步驟的占位註解，未說明 zlogger 不負責讀取
   YAML、JSON、TOML 或環境變數，也未清楚區分 decoder 的未知欄位檢查與
   `Config.Validate` 的值域驗證。

## 目標

1. README 內納入本次檢查的 Go 樣本可獨立編譯或由可執行測試覆蓋。
2. 文件對全域 logger、instance logger 與 zap global logger 的關係描述正確。
3. `color_enabled` 僅在 `format=console` 時產生 ANSI 色碼；JSON 永不因該設定含色碼。
4. 明確記錄初始化、cleanup、關閉、同步、level 回退及 sink ownership 契約。
5. 補足目前主要公開 API 與可判斷錯誤值的導覽，降低使用者閱讀原始碼的需求。
6. 保留現有 API 相容性，不要求既有呼叫端改寫初始化或分級輸出架構。
7. 明確說明設定檔整合邊界、所有欄位、預設值、合法值、條件式必填、正規化、
   錯誤分類與安全限制。
8. 在中文版 README 加入可由 GitHub 直接渲染的架構圖，讓使用者快速理解設定來源、
   logger 建立方式、標準輸出及分級輸出的責任邊界。
9. 以繁體中文版為主要契約，同步英文 README 的章節結構、程式碼、技術值、架構
   關係與公開行為，消除兩種語言的功能落差。

## 非目標

1. 不新增或替換 Gin middleware package；README 的 Gin 程式仍是整合樣本。
2. 不把 timberjack 納入 zlogger 的必要依賴，也不新增 rotation adapter。
3. 不改變 `Configure` 成功後只能設定一次的既有契約。
4. 不改變 `Init`、`SetLevel` 的公開函式簽章或 legacy 回退行為。
5. 不改變 `SplitOutput` 的每日換檔、分級路由或資源關閉實作。
6. 不建立 tag、GitHub Release 或執行發布；版本發布另由 release 流程處理。
7. 不全面改寫 README 的結構、語氣或其他未經確認的範例。
8. 不在 zlogger 內新增 YAML、JSON、TOML、Viper 或環境變數載入器。
9. 不建立獨立圖片資產。
10. 不在本次建立 `docs/zh-TW`、`docs/en` 或進行完整文件網站拆分；長篇文件拆分另立
    Docs spec。

## 現有行為與新行為

| 項目 | 現有行為 | 本次完成後 |
| --- | --- | --- |
| Gin 跳過 middleware | 樣本呼叫不存在的小寫名稱 | 呼叫 `SkipMiddlewareLog` |
| Gin 設定欄位 | 宣告三個未使用欄位 | 移除未生效欄位，僅保留樣本真正支援的設定 |
| timberjack 單檔 | 無法編譯，且混淆 zap 與 zlogger global | 樣本可編譯，直接使用建立出的 logger |
| JSON 顏色 | `color_enabled=true` 時 level 可能含 ANSI | JSON level 永遠不含 ANSI |
| Configure | 一次性契約未明示 | 明示第二次成功設定回傳 `ErrAlreadyConfigured`，cleanup 不重設資格 |
| Init | 只說不能回傳錯誤 | 明示初始化失敗可能 panic，推薦 `Configure` |
| SetLevel | 未知字串默默回退 `info` | 文件明示 legacy 回退行為 |
| 公開 API | 新 API 分散於各章節或缺漏 | 提供精簡索引與契約連結 |
| 設定檔載入 | 省略解析實作，容易誤認為 zlogger 會載入檔案 | 明示呼叫端負責解析，zlogger 負責 resolve、normalize、validate 與初始化 |
| 未知設定 key | 是否拒絕取決於外部 decoder，README 未說明 | 明示必須在 decoder 啟用嚴格模式，`Config.Validate` 無法偵測未映射 key |
| 架構導覽 | 使用者需跨多個章節拼湊標準與分級輸出流程 | README 前段提供單一 Mermaid 架構圖並連結後續細節 |
| 中英文一致性 | 中文版約 864 行、英文版約 251 行，章節與功能說明明顯不同步 | 兩份 README 採相同資訊架構與公開契約，僅自然語言不同 |

## 使用情境

### 情境一：複製 Gin 整合樣本

使用者應能複製樣本並以正確匯出名稱跳過 middleware 日誌，不會看到實際未生效的
設定欄位。

### 情境二：接入 timberjack

使用者應能選擇單檔或三檔分級 rotation；樣本必須清楚指出 logger 的實際呼叫入口，
以及 sink 的 `Sync`、`Close` 責任。

### 情境三：使用 JSON 輸出

使用者即使保留預設 `color_enabled=true`，JSON 的 `level` 仍須是一般文字，不含 ANSI
escape sequence。

### 情境四：設定全域 logger

使用者應能從 README 判斷 `Configure`、`ConfigureWithOptions`、`Init` 的錯誤處理與
生命週期差異，避免在同一 process 嘗試重新 Configure。

### 情境五：使用 instance 與 context API

使用者應能找到 `New`／`NewWithOptions`、`Instance.Close`／`Sync`、`FromContext`、
`WithOperation`、`WithComponent` 的用途與最小入口。

### 情境六：從設定檔載入

使用者應能確認 zlogger 接受的欄位名稱與值域，並了解設定檔讀取、格式解析、環境變數
覆寫及拒絕未知 key 都由呼叫端的設定工具負責。解析完成後，以 `ConfigPatch` 傳入
`Configure`，由 zlogger 套用預設值、正規化與驗證。

### 情境七：快速理解套件架構

首次使用者應能從 README 前段的架構圖判斷：設定檔由呼叫端解析、全域與 instance
logger 是不同入口、標準 console／file output 與 `SplitOutput`／`NewSplitCore` 是
不同輸出路徑，以及外部 sink 由誰管理生命週期。

### 情境八：切換中英文文件

使用者從任一 README 切換語言後，應能在相同章節位置找到相同 API、設定、樣本、
錯誤與架構資訊，不會因閱讀英文版而取得過時契約。

## 驗收情境

### AC-1：JSON 不含 ANSI 色碼

測試：`go test ./... -run 'TestBuildEncoderConfig|Test.*Color'`

假設設定為 `format=json` 且 `color_enabled=true`

當 logger 編碼任一日誌級別

那麼 JSON level 不得包含 ANSI escape sequence

### AC-2：console 顏色契約維持

測試：`go test ./... -run 'TestBuildEncoderConfig|Test.*Color'`

假設設定為 `format=console` 且 `color_enabled=true`

當 logger 編碼日誌級別

那麼 level 保持既有彩色大寫輸出

### AC-3：停用顏色維持無色

測試：`go test ./... -run 'TestBuildEncoderConfig|Test.*Color'`

假設 `color_enabled=false`

當 logger 使用 console 或 JSON encoder

那麼 level 不得包含 ANSI escape sequence

### AC-4：README 樣本可驗證

測試：`go test ./...`，必要時新增 `Example` 或文件樣本編譯測試

假設使用者依 README 使用 Gin 與 timberjack 樣本

當樣本經 Go 編譯檢查

那麼不得出現不存在的符號或未使用 import

### AC-5：全域 logger 契約正確

測試：文件檢查加既有 `TestConfigure*` 回歸測試

假設使用者閱讀初始化與 timberjack 章節

當使用者選擇 `Configure`、`Init`、直接建立 zap logger 或 `NewSplitCore`

那麼文件能正確指出 logger 呼叫入口、錯誤處理與 cleanup ownership

### AC-6：公開 API 可發現

測試：文件人工檢查及 `rg` API 名稱檢查

假設使用者需要設定、context 或分級 sink 功能

當使用者搜尋 README

那麼能找到 `ConfigureWithOptions`、`ErrAlreadyConfigured`、
`ErrInvalidSplitCore`、`FromContext`、`WithOperation`、`WithComponent`

### AC-7：設定檔契約明確

測試：文件人工檢查及 `rg` 欄位名稱檢查

假設使用者要以 YAML、JSON、TOML 或 Viper 載入 logger 設定

當使用者閱讀「從設定檔載入」與設定選項章節

那麼文件必須清楚說明解析責任、支援的 tags、九個設定欄位、預設值、合法值、
條件式驗證、大小寫正規化、pointer 零值語意及錯誤判斷方式

### AC-8：README 架構圖正確且可渲染

測試：Mermaid 語法檢查與文件人工審查

假設使用者從 GitHub 開啟中文版 README

當 GitHub 渲染架構章節

那麼圖中必須清楚分開設定解析、global／instance logger、標準輸出、每日分級輸出及
自訂 sink 輸出，且不得暗示 zlogger 內建設定檔 loader 或取得外部 sink ownership

### AC-9：中英文 README 契約一致

測試：標題、程式碼區塊、API 名稱、設定 key 與 Mermaid edge 對照檢查

假設同一使用者分別閱讀 `README.md` 與 `README.en.md`

當使用者查找安裝、設定、初始化、輸出、生命週期、錯誤或 API 導覽

那麼兩份文件必須具有相同章節順序與公開契約，程式碼及技術值一致，且語言切換連結
可正確到達對應 README

## 驗收條件

1. AC-1 至 AC-9 全部通過。
2. `go test -race -count=1 ./...` 通過。
3. `go vet ./...` 通過。
4. `git diff --check` 通過。
5. README 不再出現 `middleware.skipMiddlewareLog`。
6. timberjack 單檔樣本不再包含未使用 import 或錯誤的 zlogger global 說明。
7. README 清楚標示尚未發布版本與 release tag 的關係；實際發布不屬於本 spec。
8. README 不再使用「載入 YAML」占位註解作為唯一解析說明，且不暗示 zlogger
   自帶設定檔 loader。
9. README 含一張 GitHub Mermaid 可渲染的高階架構圖，圖中文字與程式契約一致。
10. `README.md` 與 `README.en.md` 的資訊架構、程式碼、設定表、API、錯誤值及
    Mermaid 關係一致。

## 驗證需求

1. 以 table-driven test 驗證 format 與 `color_enabled` 的組合。
2. 保留 console 彩色行為的回歸測試，避免修正 JSON 時全面關閉顏色。
3. 對 README 內本次修改的樣本執行編譯驗證；若無法直接納入 doctest，應抽成
   `example_test.go` 並使 README 與其保持一致。
4. 執行完整 race test 與 vet，確認 encoder 調整未影響其他 logger 路徑。
5. 人工檢查 README 的 resource ownership、一次性設定與錯誤值描述是否與原始碼一致。
6. 人工核對 `Config`、`ConfigPatch` 的所有 tags、預設值及 `Validate` 條件，並明確
   區分 decoder 錯誤與 `ErrInvalidConfig`。
7. 驗證 Mermaid code block 語法，並逐條核對節點與箭頭不會誤表達初始化或資源
   ownership。
8. 比對兩份 README 的標題順序、fenced code blocks、公開符號、設定 key、錯誤值與
   相對連結；自然語言允許不同，但不得有功能契約落差。
