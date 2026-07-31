# 雙語文件結構拆分需求

Status: Complete

## 文件定位

本 spec 接續 `.specs/2026-07-31-11-56_BugFix-readme-usage-contract-accuracy` 已完成的
雙語 README 契約校正與架構圖。本次只調整文件資訊架構，將長篇說明拆入
`docs/zh-TW` 與 `docs/en`。

本 spec 不重寫 logger、設定、檔案安全、context、`SplitOutput`、`SplitSinks` 或
rotation 實作，也不改變前一份 spec 已固定的公開行為。

## 背景

目前雙語 README 已對齊公開契約，但各約 600 行，快速開始、完整設定、生命週期、
安全邊界與整合範例集中於單一檔案。此結構有以下問題：

1. 首次使用者需要捲動大量進階內容才能找到主要入口。
2. 設定、生命週期與輸出 ownership 的詳細說明缺少穩定的專題連結。
3. Gin 與 timberjack 範例篇幅較長，增加 README 維護成本。
4. 未來補充進階內容時，只能繼續擴大兩份 README。
5. 中英文內容雖已同步，但缺少每個專題頁面的對應語言切換與一致導覽。

## 目標

1. 建立 `docs/zh-TW` 與 `docs/en` 的鏡像文件結構。
2. 讓 `README.md` 與 `README.en.md` 回歸專案定位、架構、安裝、快速開始、能力摘要、
   API 導覽及專題連結。
3. 將完整設定、生命週期、輸出、context、安全性及整合範例移入專題文件。
4. 每一份專題文件提供對應語言切換與語言內導覽。
5. 中英文文件維持相同檔名、章節意圖、程式碼、技術值、API 與錯誤契約。
6. 所有相對連結可從 GitHub 正確開啟，不依賴翻譯後的 heading anchor。
7. 避免內容重複：README 提供摘要，專題頁面提供完整契約與範例。

## 非目標

1. 不修改 production Go code、公開 API、設定 schema 或測試行為。
2. 不加入 Gin、timberjack、YAML、TOML 或設定框架 production dependency。
3. 不建立文件網站、GitHub Pages、MkDocs、Docusaurus 或其他 generator。
4. 不修改 CI workflow、release tag、Codecov 或 Dependabot。
5. 不建立自動翻譯流程，也不要求中英文逐字直譯。
6. 不建立完整 Go API reference；公開符號仍以 GoDoc 為權威來源。
7. 不把 SDD 歷史文件移入 `docs/`。

## 目標文件結構

```text
docs/
├── zh-TW/
│   ├── README.md
│   ├── configuration.md
│   ├── lifecycle.md
│   ├── output-modes.md
│   ├── context-and-fields.md
│   ├── security.md
│   └── integrations/
│       ├── gin.md
│       └── timberjack.md
└── en/
    ├── README.md
    ├── configuration.md
    ├── lifecycle.md
    ├── output-modes.md
    ├── context-and-fields.md
    ├── security.md
    └── integrations/
        ├── gin.md
        └── timberjack.md
```

## 內容分工

| 文件 | 保留或移入的內容 |
| --- | --- |
| 根目錄 README | 定位、架構圖、安裝、快速開始、功能摘要、精簡 API／錯誤導覽、專題連結、開發驗證 |
| docs index | 文件地圖、閱讀路徑、語言切換、GoDoc 與根 README 連結 |
| configuration | `ConfigPatch`、設定檔 loader 邊界、九個欄位、正規化、驗證、顏色 |
| lifecycle | `Configure`、`ConfigureWithOptions`、`Init`、`New`、cleanup、`Sync`、`Close` |
| output-modes | console、file、每日三檔、`SplitOutput`、`SplitSinks`、ownership 與 routing |
| context-and-fields | context API、欄位合併順序、defensive copy、field helpers |
| security | 路徑 leaf、`os.Root`、symlink、permissions、umask、`Redacted`、敏感資料 |
| integrations/gin | 呼叫端 middleware、request ID、skip、custom fields、依賴邊界 |
| integrations/timberjack | 單檔與三檔 rotation、容量／保留／壓縮、process 與 sink ownership |

## 使用情境

### 情境一：首次導入

使用者從根 README 應能在短時間內完成安裝、理解 global／instance 差異並執行最小
範例，不必先閱讀所有安全與整合細節。

### 情境二：設定檔整合

使用者可從根 README 進入對應語言的 configuration 文件，找到設定來源責任、完整欄位
表與嚴格 decoder 範例。

### 情境三：選擇輸出策略

使用者可從 output-modes 文件比較單檔、每日分級與外部 rotation，並判斷 resource
ownership 與正確 cleanup 順序。

### 情境四：整合 Gin 或 timberjack

使用者可直接進入 integration 文件取得完整、可編譯的引用端範例，而不讓根 README
承載所有第三方套件細節。

### 情境五：切換語言

使用者在任一專題頁面都能切換至相同主題的另一語言版本，並保留相同技術契約。

## 驗收情境

### AC-1：雙語文件鏡像完整

測試：文件樹與相對路徑比對

假設任一相對路徑存在於 `docs/zh-TW`

當檢查 `docs/en` 的相同相對路徑

那麼必須存在對應文件，反向檢查也相同

### AC-2：根 README 保持快速入口

測試：heading、行數與內容人工檢查

假設使用者開啟任一語言的根 README

當尋找安裝、快速開始、架構、主要能力與專題文件

那麼所有入口都可直接找到，且 Gin、timberjack 等完整長篇範例不再重複存在

### AC-3：設定文件完整

測試：技術 token 與設定表檢查

假設使用者閱讀 configuration 文件

當查找 `ConfigPatch`、九個設定欄位、預設值、合法值、未知 key 與錯誤分類

那麼兩種語言都提供相同契約與可執行方向

### AC-4：生命週期與輸出 ownership 明確

測試：API、錯誤值與 cleanup 順序檢查

假設使用者閱讀 lifecycle 與 output-modes 文件

當選擇 global、instance、`SplitOutput` 或外部 sink

那麼文件能明確指出誰負責 `Sync`、`Close`、cleanup 及關閉後錯誤

### AC-5：整合範例可編譯

測試：暫存 module 編譯 Gin、timberjack 與設定 loader 樣本

假設使用者依 integration 文件建立呼叫端程式

當使用目前文件指定的外部套件版本編譯

那麼不得出現不存在的欄位、符號或未使用 import

### AC-6：語言切換與導覽有效

測試：本地相對連結檢查

假設使用者位於任一根 README、docs index 或專題文件

當點擊語言切換、上一層或相關主題連結

那麼連結必須指向存在的相對路徑，不使用跨語言 heading anchor

### AC-7：中英文技術契約一致

測試：code fence、API、設定 key、錯誤值與命令比對

假設兩份對應語言文件描述相同主題

當比對技術內容

那麼自然語言可不同，但程式碼行為、技術值與 resource ownership 不得有差異

### AC-8：程式行為無回歸

測試：`go test -race -count=1 ./...`、`go vet ./...`

假設本次只重整文件

當執行既有測試與靜態分析

那麼所有 production behavior 與既有測試必須維持通過

## 驗收條件

1. AC-1 至 AC-8 全部通過。
2. 目標文件樹完整，無單語孤兒頁面。
3. 根 README 的雙語章節結構與技術內容一致。
4. 每份專題文件含語言切換與語言內 index 連結。
5. 所有本地 Markdown 連結指向存在的檔案。
6. Gin、timberjack 與設定 loader 範例編譯通過。
7. 未新增 production dependency 或修改 Go API。
8. `go test -race -count=1 ./...`、`go vet ./...`、Markdown 結構檢查與
   `git diff --check` 通過。

## 驗證需求

1. 先建立完整雙語文件樹，再縮減根 README，避免移動過程遺失內容。
2. 對每組對應文件比較 heading 意圖、code fence 語言序列、公開 API、設定 key、
   error sentinel、命令與相對連結。
3. 使用暫存 module 編譯涉及外部依賴的範例，不修改專案 `go.mod`。
4. 檢查所有相對 Markdown 連結；外部連結至少確認 URL 結構正確。
5. 完成後執行完整 race test、vet、既有 `make verify` 與 diff 檢查。
