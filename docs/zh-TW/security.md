# 安全性

## 建構時啟用欄位遮罩

`NewRedactingCore(encoder, sink, level, RedactionConfig{Enabled: true, Keys: []string{"token", "password"}})`
建立遮罩 core；零值 Enabled=false，保留一般 zapcore.NewCore 行為且忽略 Keys。
啟用時 Keys 須有 1 至 128 個項目，每個 1 至 128 bytes，重複項目合併；不去除空白。
採 ASCII 大小寫不敏感的 leaf name 精確匹配，不支援路徑、正則或執行中更新。
設定會複製，修改原 slice 不影響已建構 core。設定無效回傳 ErrInvalidConfig。

匹配欄位固定輸出 `[REDACTED]`，不求值原容器；匹配 namespace 遮罩整個子樹。
Object／Array／Inline 遞迴處理，Reflect、Stringer、Error 即使 key 不匹配也整欄遮罩，
不呼叫來源 String／Error／MarshalJSON。Any 依 zap 選出的 Field 型別套用同一政策。
深度超過 32、欄位與元素總預算超過 1024、marshaler 錯誤或 panic 時整筆拒絕，
回傳固定 ErrRedactionFailed，不寫出部分資料，也不附帶來源錯誤。With 的失敗延後至 Write 回報。

請提供未綁定欄位的可信任 encoder，以及有效的 sink／level；不保護既有 encoder 欄位、
message、自由文字、key 名稱、呼叫端在建立 Field 前的求值、外部 hook 或原始 I/O error。
無法中止不返回的使用者 marshaler；它仍屬可信任程式碼。遮罩不取代欄位 allowlist。
本 API 不關閉 sink，亦不建立 worker。直接 core.Write 可取得錯誤；logger 則交給 ErrorOutput。

## 故障事件的敏感資訊邊界

OutputFailure 不保存日誌本文、fields 或額外完整路徑，Err 也不參與 JSON／YAML 序列化。
但原始 error 本身仍可能帶有檔案路徑或其他內部資訊；不要直接將 `event.Err`、其字串
或 `%+v` 格式的整個事件傳到外部告警。observer 是有界的盡力通知，不是可靠事件儲存，
應監看 Dropped；不能據此宣稱每次故障都已通知。Typed nil sink 不受一般 nil 檢查保證，
呼叫端須提供有效的 WriteSyncer。

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
