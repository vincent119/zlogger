# 雙語文件結構拆分任務

Status: Complete

## Execution Context

### 意圖

將目前完整但篇幅較長的雙語 README 拆成快速入口與雙語專題文件，維持所有公開契約、
程式碼範例及 resource ownership 的正確性。

### 非目標

1. 不修改 Go production code、Go tests 或公開 API。
2. 不新增 production dependency。
3. 不建立文件網站、產生器、CI workflow 或 release。
4. 不移動 `.specs` 歷史文件。

### 已定決策

1. 繁體中文為主要契約，英文在同一 task 同步。
2. `docs/zh-TW` 與 `docs/en` 使用完全相同的相對檔名。
3. 根 README 保留 onboarding、摘要與導覽；詳細內容只在專題頁面完整保存。
4. 每份專題頁面都有對應語言切換與語言內 index 連結。
5. integrations 保持引用端程式，不讓 zlogger 增加 Gin 或 timberjack dependency。
6. 先建立並驗證 docs，再縮減根 README。

### 邊界

允許修改 `README.md`、`README.en.md`，並新增需求所列的 `docs/zh-TW`、`docs/en`
Markdown 文件。不得修改 Go source、Go tests、`go.mod`、Makefile 或 workflow。

### 關鍵檔案

1. `README.md`
2. `README.en.md`
3. `docs/zh-TW/README.md`
4. `docs/en/README.md`
5. 前一份 README 契約 spec 與目前公開 Go 實作

### 完成條件

1. requirements 的 AC-1 至 AC-8 通過。
2. T1 至 T9 全部完成。
3. 雙語文件樹、相對連結、範例編譯與完整專案驗證通過。

## Protected Behavior

1. `Configure` 一次成功與 cleanup 契約不變。
2. JSON 無 ANSI、console color 契約不變。
3. DEBUG／INFO、WARN、ERROR 以上的分級 routing 不變。
4. `Instance`、`SplitOutput` 與外部 sink ownership 不變。
5. 路徑、permissions、`os.Root` 與 symlink 安全邊界不變。
6. context fields defensive copy 與合併順序不變。
7. 最新 tag 與 `main` 的版本說明維持事實正確。

## 實作任務

### T1：建立雙語文件 index 與導航骨架

- [x] 建立 `docs/zh-TW/README.md` 與 `docs/en/README.md`。
- [x] 列出七個專題文件及建議閱讀順序。
- [x] 加入根 README、GoDoc、對應語言 index 連結。
- [x] 固定一般頁面與 integrations 頁面的導航模板。

Boundary:

- Allowed Changes：雙語 docs index；建立必要空目錄，但不建立空白專題頁面。
- Forbidden：根 README、專題內容、Go code、CI。

Depends: 無。

Context: index 是所有後續頁面的穩定入口，檔名與相對路徑需先固定。

Verify: 兩份 index 的主題清單、相對路徑與語言切換一一對應。

### T2：建立雙語設定文件

- [x] 建立兩份 `configuration.md`。
- [x] 移入 Config／ConfigPatch、loader、九欄設定表、validation 與 color 契約。
- [x] 保留嚴格 JSON decoder 完整範例。
- [x] 區分 decoder errors、`ErrInvalidConfig`、`ErrUnsafeLogPath`。

Boundary:

- Allowed Changes：雙語 `configuration.md`。
- Forbidden：根 README、其他專題、設定實作與 `go.mod`。

Depends: T1。

Context: 逐項對照 `config.go` 與目前 README；不可宣稱內建 loader。

Verify: 設定 key、預設值、合法值、code fences 與 error contract 雙語一致；JSON loader
在暫存 module 編譯通過。

### T3：建立雙語生命週期文件

- [x] 建立兩份 `lifecycle.md`。
- [x] 移入初始化入口比較、一次成功、失敗重試與 cleanup 契約。
- [x] 說明 `Init` panic、Instance Close／Sync 與 `SetLevel` fallback。
- [x] 提供正確 shutdown 呼叫順序。

Boundary:

- Allowed Changes：雙語 `lifecycle.md`。
- Forbidden：global state、API、commons/graceful dependency 或其他文件。

Depends: T1。

Context: 只文件化現有行為，不新增可重新 Configure 或 error-returning SetLevel。

Verify: `ConfigureWithOptions`、`ErrAlreadyConfigured`、`os.ErrClosed`、panic、cleanup 與
ownership 在兩份文件中一致。

### T4：建立雙語輸出模式文件

- [x] 建立兩份 `output-modes.md`。
- [x] 比較 console、file、每日分級與外部 rotation。
- [x] 移入 `GetSplitCore`、`SplitOutput`、`NewSplitCore` 範例及 routing。
- [x] 明確區分 internal cleanup 與 external sink ownership。
- [x] 連結 security 與 timberjack 專題。

Boundary:

- Allowed Changes：雙語 `output-modes.md`。
- Forbidden：`split_output.go`、完整 timberjack 範例、其他專題。

Depends: T1。

Context: 不將 `Configure`／`New` 畫成會自動建立 SplitOutput。

Verify: routing、API、錯誤值、Sync／Close 順序與交叉連結雙語一致。

### T5：建立雙語 context 與安全文件

- [x] 建立兩份 `context-and-fields.md`。
- [x] 建立兩份 `security.md`。
- [x] 移入 context defensive copy、merge order 與 field helper 分類。
- [x] 移入 safe leaf、`os.Root`、symlink、permissions、umask 與 Redacted 契約。

Boundary:

- Allowed Changes：雙語 context 與 security 四份文件。
- Forbidden：context、file security、permission production code 與其他文件。

Depends: T1。

Context: 安全限制需保留已知例外，不把 containment 描述成完整 sandbox。

Verify: API、errors、permissions、platform 限制與敏感資料規則雙語一致。

### T6：建立雙語 Gin 整合文件

- [x] 建立兩份 `integrations/gin.md`。
- [x] 移入可編譯的引用端 middleware 與 handler 用法。
- [x] 保留 SkipPaths、Context、Category、custom fields 與 request ID。
- [x] 明示時間格式與 UTC 由 encoder 管理，zlogger 不依賴 Gin。

Boundary:

- Allowed Changes：雙語 Gin integration 文件。
- Forbidden：新增 Gin package、`go.mod`、根 README 或其他專題。

Depends: T1、T3、T5。

Context: 不重新加入未使用的 TimeFormat、UTC、DefaultLevel 欄位。

Verify: 暫存 module 編譯通過；兩份文件的 Go symbols 與行為一致。

### T7：建立雙語 timberjack 整合文件

- [x] 建立兩份 `integrations/timberjack.md`。
- [x] 移入單檔及三檔 rotation 完整範例。
- [x] 記錄已驗證 timberjack 版本與 `Compression` 欄位。
- [x] 說明 `zap.ReplaceGlobals`、zlogger global、單一 process writer 與 sink ownership。

Boundary:

- Allowed Changes：雙語 timberjack integration 文件。
- Forbidden：新增 adapter、`go.mod`、SplitOutput 實作或其他專題。

Depends: T1、T4、T5。

Context: timberjack 與 SplitOutput 不得同時管理同一檔案。

Verify: 使用文件指定版本在暫存 module 編譯單檔與三檔範例；雙語 cleanup 順序一致。

### T8：縮減並同步根 README

- [x] 保留雙語架構圖、安裝、版本說明、快速開始與主要能力摘要。
- [x] 將長篇設定、生命週期、輸出、Gin、timberjack、安全內容改為摘要及專題連結。
- [x] 保留精簡 API／errors、開發驗證與 License。
- [x] 確認兩份 README heading 結構、code fences 與專題連結一致。

Boundary:

- Allowed Changes：`README.md`、`README.en.md`。
- Forbidden：刪除尚未移入 docs 的唯一內容、修改 docs 專題、Go code。

Depends: T2 至 T7。

Context: 只有對應專題完成並驗證後，才可從根 README 移除完整版本。

Verify: 根 README 可完成 onboarding；兩份文件的 headings、code fences、API、errors
與 docs link targets 一致。

## 驗證任務

### T9：完整文件與專案驗證

- [x] 比對雙語文件樹，確認沒有孤兒頁面。
- [x] 檢查每頁語言切換、index、交叉連結與本地 target。
- [x] 比對每組文件的 code fences、API、設定 key、errors 與 commands。
- [x] 編譯 JSON loader、Gin、timberjack 單檔與三檔範例。
- [x] 執行 markdownlint、race test、vet、`make verify` 與 diff 檢查。

Boundary:

- Allowed Changes：只修正 T1 至 T8 邊界內發現的文件問題。
- Forbidden：新增功能、production dependency、CI、release 或 Go behavior 變更。

Depends: T1 至 T8。

Context: 若驗證需要修改既定文件樹或 Go 程式，先更新 spec，不得直接擴張範圍。

Verify:

```bash
go test -race -count=1 ./...
go vet ./...
make verify
git diff --stat
git diff --check
```

## 品質檢查清單

- [x] `docs/zh-TW` 與 `docs/en` 相對檔案清單一致。
- [x] 每份文件都有語言切換與語言內 index 連結。
- [x] 根 README 保留可執行快速開始與完整導覽。
- [x] 沒有內容在搬移時遺失，也沒有大段重複留在 README。
- [x] 九個設定欄位與 validation 契約一致。
- [x] global、instance、SplitOutput、external sinks ownership 正確。
- [x] Gin 與 timberjack 仍是引用端整合，不是 production dependency。
- [x] 所有本地 Markdown link targets 存在。
- [x] 外部整合與 loader 範例編譯通過。
- [x] 中英文技術 tokens 與 code fence 序列一致。
- [x] 未修改 Go source、Go tests、go.mod、Makefile 或 workflow。
- [x] `make verify` 與 `git diff --check` 通過。

## Implementation Notes

2026-07-31：已在 `docs/bilingual-documentation-structure` 分支開始執行 T1。

2026-07-31：T1 至 T7 建立 16 份雙語 index 與專題文件，兩個語言目錄的相對檔案
清單一致。T8 將根 README 縮減為 162／167 行，保留相同的 8 個 H2、3 個 H3 與
5 組 code fences。

2026-07-31：T9 完成。本地 Markdown 連結、markdownlint 結構檢查、JSON loader、Gin、
timberjack v1.4.5 單檔與三檔範例均通過。`make verify` 使用 golangci-lint v2.12.2，
結果 0 issues；race test、vet、benchmark 通過，總覆蓋率 92.7%。

實作順序必須先完成 T1 至 T7，再執行 T8；避免在專題文件尚未完成前刪除 README 的
唯一內容來源。
