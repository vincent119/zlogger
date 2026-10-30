# 變更紀錄

**繁體中文** | [English](CHANGELOG.en.md)

本文件記錄 zlogger 各版本對使用者可感知的變更。格式參考
[Keep a Changelog](https://keepachangelog.com/zh-TW/1.1.0/)，版本號遵循
[Semantic Versioning](https://semver.org/lang/zh-TW/)。

<!-- markdownlint-disable MD024 -->

## [未發布]

目前沒有已記錄的變更。

## [1.2.1] - 2026-10-30

### 新增

- 新增 `Config.Redaction` 與 `ConfigPatch.Redaction`，讓一般及每日三檔建構入口直接設定 `Enabled` 與 `Keys`，不必自行組裝 zapcore。遮罩預設關閉，啟用時涵蓋所有輸出及初始化日誌。
- 遮罩設定在開檔或啟動 worker 前完成驗證，並在建構時複製 Keys；後續修改來源設定不影響既有 logger。分級、換檔、故障通知與清理契約維持不變。

### 變更

- 中英文文件與可執行範例補齊高階遮罩設定、清理方式與適用限制。

### 相容性注意事項

- `ConfigPatch.Redaction` 非 nil 時整組取代政策，不逐 key 合併。`ConfigureCore` 拒絕 `Redaction.Enabled=true` 並回傳 `ErrInvalidConfig`；外部 core 請先以 `NewRedactingCore` 組裝，再搭配 nil 或停用的高階設定。
- 使用非具名 `Config` 或 `ConfigPatch` literal 的呼叫端，需改為具名欄位。既有函式簽名與預設行為不變。
- 遮罩政策在建構時固定，不處理自由文字 message；啟用時 Error、Reflect、Stringer 等 opaque 值仍會整欄遮罩。

## [1.2.0] - 2026-10-16

### 新增

- 新增 `GetSplitCoreWithCleanup`，讓每日三檔 core 的 cleanup 回傳關檔錯誤，並支援重複與並行清理。
- 新增 `ConfigureCore`、`WithCoreCleanup` 與 `ConfigureSplit`，將自訂 core 或每日三檔 logger 接上全域 API，明確管理同步、換檔 worker 與資源所有權。
- 新增 `NewSplit` 與 `Instance.SetLevel`，讓不同元件各自持有每日三檔 logger，獨立調整級別且不修改全域設定。
- 新增 `NewFailureObserver`、`NewObservedWriteSyncer` 與 `WithOutputObserver`，觀察寫入、同步、換檔及關檔失敗。通知採有界非阻塞通道，滿載時以 `Dropped()` 累計丟棄事件；不涵蓋 core 編碼錯誤。
- 新增 `NewRedactingCore`、`RedactionConfig` 與 `ErrRedactionFailed`，支援建構時明確啟用及指定遮罩 Keys。預設關閉；啟用後處理巢狀欄位，opaque 值整欄遮罩，無法安全編碼時拒絕整筆輸出。
- 新增 `Object`、`Array`、`Inline`、`Namespace`、16 個常用切片入口，以及 `ObjectMarshaler`、`ArrayMarshaler`、`ObjectEncoder`、`ArrayEncoder` 四個型別別名。欄位 helper 共 47 個，呼叫端可只引用 zlogger 實作 marshaler，底層仍使用 zap。

### 變更

- Go 註解、套件固定錯誤與診斷、範例顯示文字統一使用英文。呼叫端傳入的訊息、欄位值與外部錯誤仍保留原文。
- 中英文文件補齊生命週期、故障通知、遮罩，以及全部 47 個欄位 helper 的用途、限制與可執行範例。

### 相容性注意事項

- 固定錯誤字串與每日換檔診斷前綴改為英文。請以 `errors.Is`／`errors.As` 判斷錯誤，避免依賴完整字串；既有告警與字串比對測試需自行確認。既有錯誤身分、包裝鏈與 API 簽名不變。
- 遮罩不處理自由文字 message，一般 logger 與 split 建構入口也不會自動啟用遮罩。原始 I/O error 與故障事件的 Err 仍可能含敏感資料，不宜直接外送。
- 新增 helper 沿用 zap 的求值、nil、namespace 與重複 key 行為，不深層複製輸入。自訂 core／encoder 的建構仍可能需要直接引用 zap。

下表列出相較 `v1.1.0` 的既有固定文字變更：

| 原固定文字 | 新固定文字 |
| --- | --- |
| logger 設定無效 | `invalid logger configuration` |
| 全域 logger 已完成設定 | `global logger is already configured` |
| 日誌檔案路徑不安全 | `unsafe log file path` |
| 檔案輸出權限無效 | `invalid log file permissions` |
| 分級 core 設定無效 | `invalid split core configuration` |
| 每日換檔失敗： | `daily log rotation failed:` |

### 安全性

- 安全掃描工具鏈改用 Go 1.26.9，Go 1.25.12 保留於相容性測試。最低 Go 版本仍為 1.25；套件更新不會替呼叫端升級工具鏈。

## [1.1.0] - 2026-07-31

### 新增

- 新增 `Configure`、`ConfigureWithOptions` 與 `ConfigPatch`，讓初始化錯誤、預設值與明確零值可以分開處理。
- 新增 `New`、`NewWithOptions` 與 `Instance`，讓非全域 logger 具備明確的 `Sync` 和 `Close` 生命週期。
- 新增 `NewSplitCore` 與 `SplitSinks`，支援呼叫端提供自訂 info、warn、error sinks。
- 新增 `WithDirPerm` 與 `WithFilePerm`，允許在安全限制內調整新建日誌目錄和檔案的權限。
- 新增 `Redacted`，以固定遮罩值記錄敏感欄位名稱，且不接收原始秘密值。
- 新增繁體中文與英文文件，涵蓋設定、生命週期、輸出模式、安全性、Gin 與 timberjack 整合。

### 變更

- 最低 Go 版本提升至 Go 1.25；CI 驗證 Go 1.25.12 與 Go 1.26.5。
- zap 依賴更新至 v1.28.0。
- 設定值會先正規化並嚴格驗證；未知 level、format、output 與重複 output 不再靜默降級。
- 新建日誌目錄與檔案的預設權限收緊為 `0700` 與 `0600`，且不修改既有物件權限。
- `ColorEnabled` 只影響 console encoder；JSON 輸出不再包含 ANSI 顏色控制碼。
- CI 新增固定版本 lint、race tests、漏洞掃描、benchmark、90% coverage gate、Codecov 與 Dependabot。

### 修正

- 修正 `SplitOutput` 每日換檔 goroutine 無法停止，以及關閉後可能重新開啟檔案的問題。
- `SplitOutput.Close` 與 `Instance.Close` 支援重複及並行呼叫，並聚合資源關閉錯誤。
- 關閉後的 `SplitOutput.Write` 與 `SplitOutput.Sync` 會回傳可由 `errors.Is` 判斷的 `os.ErrClosed`。
- 修正 DEBUG 日誌未寫入 info sink 的分級路由問題。
- 修正檔案輸出資源未完整持有、失敗路徑未依反向順序清理，以及 Windows 測試檔案未關閉的問題。
- `WithContext` 與 `FromContext` 現在複製 field slices，避免呼叫端透過別名修改 context 內部資料。
- 移除未接入正式路徑且可能改寫原始訊息的 SQL 處理 core，並校正 encoder helper 的實際契約。

### 安全性

- `file_name` 與 `filePrefix` 只接受單一 leaf name，拒絕路徑分隔符、NUL、絕對路徑與 Windows drive prefix。
- 使用 `os.Root` 限制檔案解析結果不得逸出呼叫端指定的 base directory，並拒絕既存 symlink leaf。
- 新增 `ErrUnsafeLogPath`、`ErrInvalidFilePermission`、`ErrInvalidConfig` 與其他可分類的 sentinel errors。
- GitHub Actions 使用完整 commit SHA，並加入固定版本 `govulncheck` 與依賴安全自動化。

### 已棄用

- `Init` 保留來源碼相容，但新程式應改用可回傳錯誤與 cleanup 的 `Configure`。
- `Config.Merge` 保留來源碼相容，但新程式應使用可區分未提供值的 `ConfigPatch.Resolve`。
- `NewNoEscapeJSONEncoder` 與 `DisableHTMLEscaping` 保留來源碼相容；新程式應直接使用 zap encoder API。

### 相容性注意事項

- 使用 Go 1.24 或更早版本的專案必須先升級工具鏈。
- 過去在 `file_name` 或 `filePrefix` 中傳入子目錄或完整路徑的設定會被拒絕；目錄應改由 `log_path` 或建構參數提供。
- `NewSplitCore` 不關閉外部 sinks；呼叫端仍須執行最終 `Sync` 與 `Close`。

## [1.0.5] - 2026-01-09

### 變更

- 新增英文 README，並統一 Go 程式碼註解與格式。

## [1.0.4] - 2026-01-09

### 新增

- 新增 Makefile、GitHub Actions CI 與專案狀態徽章。
- 建立測試覆蓋率報告流程。

### 變更

- 測試覆蓋率提升至 91.5%。

## [1.0.3] - 2025-12-11

### 新增

- 設定 console encoder 的欄位分隔符，使純文字日誌格式更清楚。

## [1.0.2] - 2025-12-10

### 新增

- 新增 `ColorEnabled` 設定，允許控制 console 日誌的 level 顏色。

<!-- markdownlint-enable MD024 -->

[未發布]: https://github.com/vincent119/zlogger/compare/v1.2.1...HEAD
[1.2.1]: https://github.com/vincent119/zlogger/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/vincent119/zlogger/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/vincent119/zlogger/compare/v1.0.5...v1.1.0
[1.0.5]: https://github.com/vincent119/zlogger/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/vincent119/zlogger/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/vincent119/zlogger/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/vincent119/zlogger/releases/tag/v1.0.2
