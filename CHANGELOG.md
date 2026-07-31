# 變更紀錄

**繁體中文** | [English](CHANGELOG.en.md)

本文件記錄 zlogger 各版本對使用者可感知的變更。格式參考
[Keep a Changelog](https://keepachangelog.com/zh-TW/1.1.0/)，版本號遵循
[Semantic Versioning](https://semver.org/lang/zh-TW/)。

<!-- markdownlint-disable MD024 -->

## [未發布]

目前沒有已記錄的變更。

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

[未發布]: https://github.com/vincent119/zlogger/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/vincent119/zlogger/compare/v1.0.5...v1.1.0
[1.0.5]: https://github.com/vincent119/zlogger/compare/v1.0.4...v1.0.5
[1.0.4]: https://github.com/vincent119/zlogger/compare/v1.0.3...v1.0.4
[1.0.3]: https://github.com/vincent119/zlogger/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/vincent119/zlogger/releases/tag/v1.0.2
