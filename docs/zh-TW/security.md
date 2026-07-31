# 安全性

**繁體中文** | [English](../en/security.md)

[返回文件首頁](README.md)

## 路徑邊界

`log_path` 與 `SplitOutput` 的 directory 是呼叫端信任並選定的 base directory。
`file_name` 與 `filePrefix` 只允許單一 leaf name，不得為 `.`、`..`，不得包含 `/`、
`\`、NUL、絕對路徑、Windows drive prefix 或其他路徑語意。

不安全名稱可由 `errors.Is(err, zlogger.ErrUnsafeLogPath)` 判斷。經 Config 驗證時，錯誤鏈
同時保留 `ErrInvalidConfig`。

## os.Root containment

每批檔案先以 `os.OpenRoot` 開啟 base directory，再以 root-relative leaf 執行 `Lstat`
與 `OpenFile`。穩定存在的最終 symlink 會被拒絕；檢查後若 leaf 被替換，`os.Root` 仍
阻止解析結果逸出 root。

此機制不是完整 filesystem sandbox：

- `OpenRoot` 會跟隨 base path 本身的 symlink。
- 不阻止 mount boundary、bind mount、特殊裝置或惡意 filesystem。
- 競態中指向 root 內部的 symlink 可能被跟隨。
- Go `js`、`plan9`、`wasip1` 有標準庫限制；驗收平台是 Linux、macOS、Windows。

呼叫端仍須保護 base directory 與設定來源。

## 建立權限

新建目錄與檔案預設使用 `0700` 與 `0600`。umask 可進一步收緊，既有物件不會被
chmod。需要放寬時：

```go
instance, err := zlogger.NewWithOptions(
	cfg,
	zlogger.WithDirPerm(0o750),
	zlogger.WithFilePerm(0o640),
)
```

目錄必須包含 owner `rwx`，檔案必須包含 owner `rw`；不得含 other-write 或非
permission bits。無效值回傳 `ErrInvalidFilePermission`。同類 option 最後一個值生效。

options 只影響新建物件，不繞過 umask，也不修改既有權限。Windows 可呼叫相同 API，
但不保證可觀察的 POSIX mode 語意。權限不開放為設定檔欄位，避免未受信任的外部設定
直接放寬 filesystem 存取。

## 敏感資料

採用欄位 allowlist，只記錄診斷所需的最小資料。不要記錄 token、API key、密碼、
私鑰、Authorization、cookie、session identifier、完整個資，或含秘密欄位的完整
request、response、Config 與任意 struct。

```go
zlogger.Info("驗證請求",
	zlogger.Redacted("authorization"),
	zlogger.String("request_id", requestID),
)
```

`Redacted` 只輸出固定 `[REDACTED]`，不掃描或自動遮罩其他欄位。
