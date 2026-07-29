# 需求文件：Encoder 契約與 SQL dead code 清理

## 來源

- Draft：無
- Type：Refactor
- Owner：待確認
- Status：InProgress
- 前置規格：`.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/tasks.md` 的後續改善

## 文件定位

本 spec 接續前置安全規格的「清理 encoder 假契約與 SQL dead code」待辦。只處理兩個既有 encoder 相容函式、未接入產品流程的 SQL core/helper、相關測試，以及 DESIGN 中與實作不一致的宣稱；不重寫 logger 建立流程、Config、Context、file output、SplitOutput 或 zap dependency。

參考來源：

- 需求來源：使用者確認的下一項改善
- 既有文件：`DESIGN.md` 的 SQL 處理與 zap 差異章節
- 既有程式碼：`encoder.go`、`encoder_test.go`、`core.go`、`core_test.go`
- 版本契約：repository 已有 `v1.0.2` 至 `v1.0.5` tags，匯出 API 不在本次直接移除
- Dependency 契約：`go.mod` 固定 `go.uber.org/zap v1.27.0`

## 背景

`NewNoEscapeJSONEncoder` 只是 `zapcore.NewJSONEncoder` 的 wrapper。zap v1.27.0 的 JSON encoder 本身就不進行瀏覽器／JSONP 防護型 HTML escaping，因此函式名稱描述的可觀察輸出成立，但沒有提供 zlogger 特有能力。

`DisableHTMLEscaping` 對已建立 logger 加入永遠回傳 nil 的 hook。hook 無法更換既有 core 的 encoder，也不會改變 escaping，只增加一個每筆日誌都會執行的無作用 callback。現有測試只檢查回傳值非 nil，沒有驗證名稱宣稱的行為。

`sqlProcessingCore` 與 `processSQLString` 只被 package 內測試直接呼叫，沒有接入 `New`、`Configure`、一般 core 或 SplitOutput。DESIGN 卻宣稱 zlogger 會自動清理 SQL 轉義字元，形成不存在的產品契約。此外，該 core 會原地修改 fields，並移除所有 message 反斜線，不適合在未定義輸入格式時接入正式流程。

## 問題陳述

目前 encoder API 的文件讓使用者誤以為 zlogger 能改變任意 logger 的 HTML escaping，而 SQL 設計文件宣稱了一項產品實際未提供且可能破壞原始資料的功能。冗餘與 dead code 增加維護成本，也掩蓋真實資料保真契約。

## 目標

1. 以可執行測試固定 zap JSON encoder 對 `<`、`>`、`&` 的實際行為。
2. 對兩個既有匯出 encoder helper 加入 Go 可辨識的 `Deprecated:` 說明，不在 v1 直接移除。
3. 將 `DisableHTMLEscaping` 改為明確無副作用的 compatibility no-op，不再安裝無作用 hook。
4. 移除未接入產品流程的 `sqlProcessingCore`、`processSQLString` 與只驗證 dead code 的測試。
5. 修正 DESIGN，明確說明 zlogger 不改寫 message 或名為 `sql` 的欄位內容。
6. 保持公開 API 可編譯，並維持 logger、encoder、輸出與 level routing 的既有行為。

## 非目標

1. 不在 v1.x 移除 `NewNoEscapeJSONEncoder` 或 `DisableHTMLEscaping` 符號。
2. 不自行實作 JSON encoder 或 fork zap。
3. 不新增 SQL formatter、parser、redaction 或 query normalization。
4. 不改寫呼叫端傳入的 message、SQL、path、JSON 或其他字串。
5. 不修改 Config schema、公開 logger API、Context、file output 或 SplitOutput。
6. 不更新 zap 或其他 dependency。
7. 不處理 README.en 維護策略。

## 已定決策

- `NewNoEscapeJSONEncoder` 保留為 deprecated compatibility wrapper，replacement 為 `zapcore.NewJSONEncoder`。
- `DisableHTMLEscaping` 保留簽章，改為原樣回傳 logger；nil 輸入回傳 nil。
- `DisableHTMLEscaping` 的 replacement 是在建立 core 時直接選擇符合需求的 encoder，不能在 logger 建立後切換。
- SQL dead code 全部移除，不將其接入正式流程。
- zlogger 對 message 與 fields 採資料保真，不根據 `field.Key == "sql"` 做隱含改寫。
- DESIGN 第 6 節改為 encoder 相容性與資料保真契約，避免大範圍重編後續章節編號。

## 待確認項目

- 無。

## 現有行為

- `NewNoEscapeJSONEncoder` 回傳 zap JSON encoder，但測試只檢查非 nil。
- `DisableHTMLEscaping` 回傳加上無作用 hook 的新 logger，無法修改 encoder。
- `DisableHTMLEscaping(nil)` 會因 nil receiver 呼叫而 panic。
- SQL helper/core 沒有產品呼叫點，但 package 測試使其看似受支援。
- DESIGN 宣稱 zlogger 自動清理 SQL 轉義字元，實際 logger 不會執行。

## 新行為

- `NewNoEscapeJSONEncoder` 仍回傳 zap JSON encoder，測試驗證 HTML 字元保持原字元，godoc 標記 deprecated replacement。
- `DisableHTMLEscaping` 不新增 hook，對非 nil logger 回傳同一 pointer，對 nil 回傳 nil，godoc 說明其無法修改既有 encoder。
- package 內不再存在 SQL 特殊處理 core/helper 或相應 dead-code tests。
- DESIGN 明確說明 encoder 決定 JSON escaping，zlogger 不隱含改寫 SQL 或 message。

## 影響範圍

- 使用者：直接呼叫兩個 encoder helper 的使用者，以及依賴 DESIGN 理解 SQL 行為的使用者
- 功能：encoder compatibility helper、內部 core dead code、設計文件
- API / CLI：兩個公開函式簽章保留並標記 deprecated；無 CLI
- Data / Storage：不修改任何儲存格式；強化字串資料保真契約
- 文件 / 安裝 / 發布：更新 DESIGN 與 godoc；未來 major version 才評估移除 deprecated API

## 使用情境

- 作為現有使用者，我想讓 v1 升級後既有 encoder helper 仍可編譯，以便漸進遷移。
- 作為新使用者，我想從 godoc 得知正確 replacement，而不誤以為 logger 建立後仍能切換 escaping。
- 作為日誌使用者，我想讓 SQL 與 message 保持原始內容，以免 library 隱含移除合法反斜線或引號。
- 作為維護者，我想移除沒有產品呼叫點的 core 與測試，以降低錯誤契約的維護成本。

## 驗收情境

### 情境：JSON encoder 保留 HTML 字元

- 場景：message 與 string field 含 `<`、`>`、`&`
- 測試：`TestNewNoEscapeJSONEncoderPreservesHTMLCharacters`
- 假設：使用既有 `NewNoEscapeJSONEncoder` 與有效 EncoderConfig
- 當：encode 一筆 entry 與欄位
- 那麼：輸出是有效 JSON，且上述 HTML 字元不轉為 `\u003c`、`\u003e`、`\u0026`

### 情境：deprecated wrapper 與 zap 行為一致

- 場景：相同 EncoderConfig 與 entry 分別使用 zlogger wrapper 與 zap encoder
- 測試：`TestNewNoEscapeJSONEncoderMatchesZap`
- 假設：輸入包含需正常 JSON escaping 的 quote、backslash 與控制字元
- 當：兩個 encoder 分別 encode
- 那麼：輸出內容相同，wrapper 沒有額外轉換

### 情境：DisableHTMLEscaping 為明確 no-op

- 場景：既有 logger 已完成建立
- 測試：`TestDisableHTMLEscapingReturnsOriginalLogger`
- 假設：傳入非 nil `*zap.Logger`
- 當：呼叫 `DisableHTMLEscaping`
- 那麼：回傳同一 logger pointer，不新增 hook 或改變輸出

### 情境：DisableHTMLEscaping 接受 nil

- 場景：相容 helper 收到 nil logger
- 測試：`TestDisableHTMLEscapingNilLogger`
- 假設：輸入為 nil
- 當：呼叫函式
- 那麼：不 panic 並回傳 nil

### 情境：SQL dead code 完整移除

- 場景：package 產品碼與測試完成清理
- 測試：程式碼檢查
- 假設：SQL core 從未接入產品建立流程
- 當：搜尋 `.go` 檔案
- 那麼：`sqlProcessingCore`、`processSQLString` 與對應測試不存在

### 情境：既有 logger 行為不被破壞

- 場景：建立 console、file、split 與 context logger
- 測試：既有完整 test suite
- 假設：使用既有 Config 與公開 API
- 當：執行完整 race 與回歸測試
- 那麼：輸出、level routing、rotation、Sync、Close、Context 與安全邊界維持不變

## 驗收條件

1. 兩個公開 encoder helper 符號仍存在且簽章不變。
2. godoc 含 Go 工具可辨識的 `Deprecated:` marker 與具體 replacement。
3. encoder behavior tests 驗證 HTML 字元與 zap v1.27.0 一致性。
4. `DisableHTMLEscaping` 對非 nil logger 回傳同一 pointer，對 nil 回傳 nil。
5. 產品與測試 `.go` 檔不再包含 SQL core/helper 名稱。
6. DESIGN 不再宣稱自動清理 SQL，並說明資料保真契約。
7. Go 1.25.11／1.26.5 race、目標測試 20 次、`make verify` 與 coverage >= 90% 通過。

## 驗證需求

- Unit：`go test -count=1 -run 'Test(NewNoEscapeJSONEncoder|DisableHTMLEscaping)' ./...`
- 穩定性：encoder selectors `-count=20`
- Race：Go 1.25.11 與 Go 1.26.5 執行 `go test -race -count=1 ./...`
- Dead code：`rg -n 'sqlProcessingCore|processSQLString' --glob '*.go' .` 無結果
- Deprecated API：`go doc . NewNoEscapeJSONEncoder` 與 `go doc . DisableHTMLEscaping` 顯示 replacement
- 品質：`make verify`
- 文件與邊界：`git diff --stat`、`git diff --check`、DESIGN 契約搜尋

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | 使用者依賴 `DisableHTMLEscaping` 回傳不同 pointer | 文件定義其為 compatibility no-op；測試固定同 pointer 行為 |
| 風險 | nil 從 panic 改為 nil 被視為行為變更 | 明確列入驗收；變更只擴大安全輸入，不移除成功路徑 |
| 風險 | 使用者誤認 deprecated wrapper 會永遠符合未來 zap 行為 | godoc 指向直接使用 pinned zap encoder；behavior test 綁定目前 dependency |
| 風險 | 移除 SQL helper 被誤認為移除產品功能 | 呼叫點檢查證明未接入；PR 明確說明只移除 dead code 與錯誤文件 |
| 假設 | zap v1.27.0 JSON encoder 不做 HTML browser escaping | 依 pinned source與 behavior test 驗證，不依記憶推測 |

## 摘要

- 關鍵決策：保留並 deprecate 匯出 helper、移除無作用 hook、刪除 SQL dead code、建立資料保真契約
- 待確認項目：無
- 風險：極少數呼叫端可能比較 logger pointer；deprecated API 最終移除需留到 major version
- 下一步：使用者確認後先建立 encoder characterization／Red tests，再依 tasks 實作
