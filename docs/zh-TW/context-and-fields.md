# Context 與 fields

## 自動遮罩與欄位綁定

`NewRedactingCore` 在建構時以 `RedactionConfig.Enabled` 與 `Keys` 固定政策，預設關閉。
先建構遮罩 core，再呼叫 With／WithLazy、Named、Sugar 或 ConfigureCore；context 合併後
的 fields 同樣受保護，不改既有欄位順序與重複 key 契約。手動 Redacted 不受開關影響。
key 依 leaf name 比較，不使用 `parent.token` 路徑語法；opaque 值整欄遮罩。
保護範圍與失敗行為見[安全性](security.md)。

**繁體中文** | [English](../en/context-and-fields.md)

[返回文件首頁](README.md)

## Request-scoped fields

```go
ctx := zlogger.WithRequestID(context.Background(), "req-123")
ctx = zlogger.WithUserID(ctx, 12345)
ctx = zlogger.WithTraceID(ctx, "trace-abc")
ctx = zlogger.WithOperation(ctx, "login")
ctx = zlogger.WithComponent(ctx, "auth")

zlogger.InfoContext(ctx, "處理請求", zlogger.String("action", "login"))
fields := zlogger.FromContext(ctx)
```

空字串不會加入 request ID、trace ID、operation 或 component；nil user ID 也不會加入。
`WithContext` 可加入任意 fields。

## 複製與合併契約

`WithContext` 會複製輸入 slice，`FromContext` 也回傳 defensive copy。呼叫端無法透過
共享底層陣列修改 context 內部欄位。

`DebugContext`、`InfoContext`、`WarnContext`、`ErrorContext`、`FatalContext` 先放入
context fields，再附加呼叫點 fields。相同 key 是否覆蓋由 zap encoder／consumer 的
處理方式決定；應避免刻意產生重複 key。

## Field helpers

| 類別 | Helpers |
| --- | --- |
| 字串 | `String`、`Strings`、`ByteString` |
| 整數 | `Int`、`Int8`、`Int16`、`Int32`、`Int64` |
| 無號整數 | `Uint`、`Uint8`、`Uint16`、`Uint32`、`Uint64` |
| 浮點與布林 | `Float32`、`Float64`、`Bool` |
| 時間 | `Duration`、`Time` |
| 錯誤 | `Err`、`NamedError` |
| 其他 | `Any`、`Binary`、`Reflect`、`Stringer`、`Stack`、`StackSkip` |

記錄任意 struct 前先確認不含秘密值；敏感資料規則請參閱[安全性](security.md)。
