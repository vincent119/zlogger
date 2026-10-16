# Context 與欄位

**繁體中文（台灣）** | [English](../en/context-and-fields.md)

[文件首頁](README.md)

欄位名稱由應用程式決定。zlogger 不要求 service、env 或 trace_id 等必填欄位，
也不會自動讀取 OpenTelemetry span。現有 helper 只是少寫幾行程式的便利入口。

## 傳入自己的欄位

以下片段假設全域 logger 已完成設定。ctx 使用請求本身的 context。

```go
ctx = zlogger.WithContext(ctx,
	zlogger.String("tenant", "example"),
	zlogger.String("job", "import"),
)
zlogger.InfoContext(ctx, "start import", zlogger.Int("batch_size", 100))
```

固定的元件欄位也可以用 `zlogger.With(fields...)` 綁在衍生 logger。
InfoContext 等套件函式仍使用全域 logger，不會自動改用你持有的獨立 Instance。
獨立 logger 可以用 `instance.Logger().With(zlogger.FromContext(ctx)...)` 帶入欄位。

## 現有 helper 的名稱

| Helper | 寫入的 key | 不加入欄位的情況 |
| --- | --- | --- |
| `WithRequestID` | `request_id` | 空字串 |
| `WithTraceID` | `trace_id` | 空字串 |
| `WithOperation` | `operation` | 空字串 |
| `WithComponent` | `component` | 空字串 |
| `WithUserID` | `user_id` | nil |

例如團隊使用 req_id，就直接用 `WithContext(ctx, String("req_id", value))`。
不需要同時加入 request_id，也不必為了使用 zlogger 改掉既有查詢欄位。
helper 不會驗證 ID 是否符合任何追蹤格式。

## 複製與合併方式

`WithContext` 會複製傳入的欄位 slice，`FromContext` 也會回傳 slice 副本。
這是淺層複製。Field 內引用的 map、slice、pointer 或物件仍由呼叫端管理，
不能把它當成深層不可變資料。

InfoContext 等入口先放 context 欄位，再附加這次呼叫的欄位。
zlogger 不去重，也不保證後面的同名 key 一定覆寫前面。
實際結果取決於 encoder 和下游解析器，請避免依賴重複 key 的順序來表達覆寫政策。

## 欄位函式說明與範例

fields.go 現有 47 個函式。下表先列原有的 27 個，後段補充 20 個結構化與切片入口。每個函式都回傳 Field，
要把結果傳給 Info、With 等入口才會寫入日誌。
範例中的 readErr、cacheErr、at、endpoint 在下方完整程式中定義。
表格的 JSON 說明以一般 zap JSON encoder、未啟用自動遮罩為前提。

| 函式 | 用途與行為 | 呼叫範例 |
| --- | --- | --- |
| `String` | 字串，JSON 輸出為字串。 | `zlogger.String("status", "ready")` |
| `Strings` | 字串切片，JSON 輸出為陣列。 | `zlogger.Strings("tags", []string{"api", "worker"})` |
| `Int` | 使用 Go int 的有號整數。 | `zlogger.Int("attempt", 2)` |
| `Int8` | 8 位元有號整數。 | `zlogger.Int8("delta8", int8(-8))` |
| `Int16` | 16 位元有號整數。 | `zlogger.Int16("delta16", int16(-16))` |
| `Int32` | 32 位元有號整數。 | `zlogger.Int32("delta32", int32(-32))` |
| `Int64` | 64 位元有號整數。 | `zlogger.Int64("delta64", int64(-64))` |
| `Uint` | 使用 Go uint 的無號整數。 | `zlogger.Uint("count", uint(2))` |
| `Uint8` | 8 位元無號整數。 | `zlogger.Uint8("count8", uint8(8))` |
| `Uint16` | 16 位元無號整數。 | `zlogger.Uint16("count16", uint16(16))` |
| `Uint32` | 32 位元無號整數。 | `zlogger.Uint32("count32", uint32(32))` |
| `Uint64` | 64 位元無號整數。 | `zlogger.Uint64("count64", uint64(64))` |
| `Float32` | 32 位元浮點數。 | `zlogger.Float32("ratio32", float32(0.5))` |
| `Float64` | 64 位元浮點數。 | `zlogger.Float64("ratio64", 0.75)` |
| `Bool` | 布林值，JSON 輸出 true 或 false。 | `zlogger.Bool("cached", true)` |
| `Err` | 錯誤欄位，key 固定為 error。nil 不產生欄位。 | `zlogger.Err(readErr)` |
| `NamedError` | 使用自訂 key 的錯誤欄位。nil 不產生欄位。 | `zlogger.NamedError("cache_error", cacheErr)` |
| `Any` | 依實際型別選擇 zap 欄位，無對應型別時使用反射。 | `zlogger.Any("payload", map[string]int{"items": 3})` |
| `Duration` | 時間長度，由 EncodeDuration 決定表示方式。 | `zlogger.Duration("elapsed", 1500*time.Millisecond)` |
| `Time` | 時間點，由 EncodeTime 決定表示方式。 | `zlogger.Time("at", at)` |
| `Binary` | 二進位資料，zap JSON encoder 以 Base64 輸出。 | `zlogger.Binary("blob", []byte{0x01, 0x02, 0x03})` |
| `ByteString` | 將位元組視為 UTF-8 文字，不是 Base64。 | `zlogger.ByteString("text", []byte("hello"))` |
| `Stringer` | 編碼欄位時呼叫 String()，不是建立 Field 時立即呼叫。 | `zlogger.Stringer("endpoint", endpoint)` |
| `Reflect` | 交給 encoder 的反射編碼途徑，可處理 map、struct 等資料。 | `zlogger.Reflect("options", map[string]bool{"dry_run": true})` |
| `Stack` | 立即擷取目前 goroutine 的堆疊，存為字串欄位。 | `zlogger.Stack("stack")` |
| `StackSkip` | 擷取堆疊時依 skip 略過上層呼叫框架。 | `zlogger.StackSkip("caller_stack", 1)` |
| `Redacted` | 固定輸出 [REDACTED]，不接受原始秘密值。 | `zlogger.Redacted("authorization")` |

### 空值、型別與編碼差異

Err(nil) 和 NamedError(key, nil) 會省略欄位，Any(key, nil) 在一般 JSON encoder 下則寫出 null。
Strings(key, nil) 寫出空陣列。這裡的 nil 指真正為 nil 的介面，
不是裝著 nil pointer 的非 nil 介面，後者要依具體型別另外處理。

Binary 適合原始位元組，ByteString 適合 UTF-8 文字。
例如表格的 blob 會得到 `"AQID"`，text 則是 `"hello"`。
Any 接到 []byte 時會選 Binary，因此也會輸出 Base64。
有二進位或 UTF-8 格式需求時，直接選對應函式，比依賴 Any 猜測更清楚。

Any 會先辨識常見純量、切片、error、Stringer，以及 zap 的 ObjectMarshaler／ArrayMarshaler。
一般 map 或 struct 沒有對應的特殊介面時，會走 Reflect。
Reflect 也可能執行物件的自訂序列化方法，不代表它能安全地記錄任意資料。
傳入的 slice、map 或物件不會因此得到深層複本，寫入前不要任意修改共享內容。

JSON 的整數欄位不保證下游也用相同寬度的整數解析。
如果收集端用浮點數處理 JSON number，大型 Int64／Uint64 識別碼就有精度風險。
需要逐字保留的識別碼，可以由應用端明確轉成字串。
Float32 本身的精度也不會因寫入 JSON 而增加。

Time 和 Duration 的值由 encoder 格式化，不保證一律是 RFC3339 或毫秒。
下方範例明確選用 RFC3339 與字串 duration，所以 at 是
`"2026-10-16T09:00:00Z"`，elapsed 是 `"1.5s"`。

### 錯誤、Stringer 與堆疊的求值時機

一般 core 會在編碼時取得 error 或 Stringer 的文字。
With 綁定欄位也可能觸發編碼，不能假設一定等到下一筆日誌才會呼叫 String／Error。
部分錯誤型別還會輸出額外的詳細資訊欄位，記錄前要先確認不含敏感資料。

Stack 和 StackSkip 則在建立 Field 時就擷取堆疊，即使該級別最後沒寫出，也已經付出成本。
skip 使用非負整數。實際第一個框架也受 zlogger 或應用端包裝函式影響，
調整時請對照輸出，不要把它和 logger 的 AddCallerSkip 當成同一個設定。

已有 logger 且只在 DEBUG 開啟時需要堆疊，可以先檢查：

```go
if entry := logger.Check(zap.DebugLevel, "diagnostic details"); entry != nil {
	entry.Write(zlogger.Stack("stack"))
}
```

這段沿用已建立的 logger，需匯入 zap 與 zlogger。
它只會在 Check 接受該筆日誌後建立 Stack 欄位。

### 可直接執行的完整範例

將以下內容存成 main.go，在已引用 zlogger 的 module 中執行 `go run .`。
程式寫入記憶體後印出 JSON，不建立檔案，也不設定全域 logger。
第一筆涵蓋全部 27 個函式，後兩筆展示 nil error 與空值。

```go
package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/vincent119/zlogger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	var output bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "msg",
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	})
	logger := zap.New(zapcore.NewCore(encoder, zapcore.AddSync(&output), zap.DebugLevel))
	readErr := errors.New("read failed")
	cacheErr := errors.New("cache miss")
	at := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	endpoint := &url.URL{Scheme: "https", Host: "example.com"}

	logger.Info("field examples",
		zlogger.String("status", "ready"),
		zlogger.Strings("tags", []string{"api", "worker"}),
		zlogger.Int("attempt", 2),
		zlogger.Int8("delta8", int8(-8)),
		zlogger.Int16("delta16", int16(-16)),
		zlogger.Int32("delta32", int32(-32)),
		zlogger.Int64("delta64", int64(-64)),
		zlogger.Uint("count", uint(2)),
		zlogger.Uint8("count8", uint8(8)),
		zlogger.Uint16("count16", uint16(16)),
		zlogger.Uint32("count32", uint32(32)),
		zlogger.Uint64("count64", uint64(64)),
		zlogger.Float32("ratio32", float32(0.5)),
		zlogger.Float64("ratio64", 0.75),
		zlogger.Bool("cached", true),
		zlogger.Err(readErr),
		zlogger.NamedError("cache_error", cacheErr),
		zlogger.Any("payload", map[string]int{"items": 3}),
		zlogger.Duration("elapsed", 1500*time.Millisecond),
		zlogger.Time("at", at),
		zlogger.Binary("blob", []byte{0x01, 0x02, 0x03}),
		zlogger.ByteString("text", []byte("hello")),
		zlogger.Stringer("endpoint", endpoint),
		zlogger.Reflect("options", map[string]bool{"dry_run": true}),
		zlogger.Stack("stack"),
		zlogger.StackSkip("caller_stack", 1),
		zlogger.Redacted("authorization"),
	)
	logger.Info("no error", zlogger.Err(nil), zlogger.NamedError("cache_error", nil))
	logger.Info("empty values", zlogger.Strings("tags", nil), zlogger.Any("missing", nil))

	if err := logger.Sync(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Print(output.String())
}
```

堆疊的路徑、行號與函式名稱依執行環境而異，不適合逐字比對。
未啟用自動遮罩時，最後兩筆的欄位部分分別只剩 message，
以及 `"tags":[],"missing":null`。

## 結構化欄位與常用切片

以下 20 個入口沿用 zap 同名函式，搭配前表共 47 個欄位函式。
`profile`、`codes` 的定義見下方完整範例，`at` 沿用前一個範例的時間值。
Any 原本就能辨識 ObjectMarshaler 與 ArrayMarshaler；新增入口是讓意圖明確，
不是新增另一套物件編碼機制。

| 函式 | 用途與行為 | 呼叫範例 |
| --- | --- | --- |
| `Object` | 以 ObjectMarshaler 編碼巢狀物件，不使用反射。 | `zlogger.Object("profile", profile{"reader"})` |
| `Array` | 依 ArrayMarshaler 提供的順序編碼陣列。 | `zlogger.Array("items", codes{3, 1})` |
| `Inline` | 把物件欄位展開至目前層級，不去除重複 key。 | `zlogger.Inline(profile{"inline"})` |
| `Namespace` | 將後續欄位放入指定子物件。 | `zlogger.Namespace("request")` |
| `Bools` | bool 切片，維持元素順序。 | `zlogger.Bools("values", []bool{true, false})` |
| `Ints` | int 切片，維持元素順序。 | `zlogger.Ints("values", []int{-2, 1})` |
| `Int8s` | int8 切片，維持元素順序。 | `zlogger.Int8s("values", []int8{-2, 1})` |
| `Int16s` | int16 切片，維持元素順序。 | `zlogger.Int16s("values", []int16{-2, 1})` |
| `Int32s` | int32 切片，維持元素順序。 | `zlogger.Int32s("values", []int32{-2, 1})` |
| `Int64s` | int64 切片，維持元素順序。 | `zlogger.Int64s("values", []int64{-2, 1})` |
| `Uints` | uint 切片，維持元素順序。 | `zlogger.Uints("values", []uint{2, 1})` |
| `Uint8s` | uint8 數字陣列，與 ByteStrings、Binary 不同。 | `zlogger.Uint8s("values", []uint8{2, 1})` |
| `Uint16s` | uint16 切片，維持元素順序。 | `zlogger.Uint16s("values", []uint16{2, 1})` |
| `Uint32s` | uint32 切片，維持元素順序。 | `zlogger.Uint32s("values", []uint32{2, 1})` |
| `Uint64s` | uint64 切片，維持元素順序。 | `zlogger.Uint64s("values", []uint64{2, 1})` |
| `Float32s` | float32 切片，維持元素順序。 | `zlogger.Float32s("values", []float32{1.5, -2.25})` |
| `Float64s` | float64 切片，維持元素順序。 | `zlogger.Float64s("values", []float64{1.5, -2.25})` |
| `Durations` | 時長切片，單位與格式由 EncodeDuration 決定。 | `zlogger.Durations("values", []time.Duration{time.Second, 1500 * time.Millisecond})` |
| `Times` | 時間切片，格式由 EncodeTime 決定。 | `zlogger.Times("values", []time.Time{at})` |
| `ByteStrings` | UTF-8 文字陣列，不是 Base64 或數字陣列。 | `zlogger.ByteStrings("values", [][]byte{[]byte("alpha"), nil, {}})` |

### Marshaler 與 encoder 別名

四個介面使用真正的 Go 型別別名，與 zapcore 的型別相同，不是另建一套介面。

| 別名 | 對應介面與方法 |
| --- | --- |
| `ObjectMarshaler` | `zapcore.ObjectMarshaler`；`MarshalLogObject(ObjectEncoder) error` |
| `ArrayMarshaler` | `zapcore.ArrayMarshaler`；`MarshalLogArray(ArrayEncoder) error` |
| `ObjectEncoder` | `zapcore.ObjectEncoder`，完整保留 AddString、AddArray、AddObject 等方法 |
| `ArrayEncoder` | `zapcore.ArrayEncoder`，完整保留 AppendInt、AppendArray、AppendObject 等方法 |

因此，欄位與 marshaler 的呼叫端可以只引用 zlogger。
這不表示底層已移除 zap；需要自行組裝 core 或選擇 encoder 時，仍可直接使用 zap。

### 求值、nil 與作用範圍

Object、Array、Inline 在建立 Field 時不呼叫 marshaler，核心編碼時才會求值。
With 可能在綁定欄位時編碼，WithLazy 的求值時機由 zap 控制。
helper 不複製物件或切片，資料在編碼前須保持穩定，並行存取由呼叫端同步。

新增切片入口在一般 JSON encoder 下，nil 與空切片都輸出 `[]`。
ByteStrings 的 nil 元素會編成空字串，Uint8s 則保留數字。
不額外處理 nil marshaler；介面持有 nil 指標的行為由其具體方法與原核心決定，
不能把切片的 nil 行為套用到 Object、Array 或 Inline。

Namespace 影響其後的欄位，包括衍生 logger 在呼叫時追加的欄位。
由 With 建立的子 logger 不會污染父 logger。
Inline 也可能引入同名 key，封裝不會去重或承諾覆寫；下游 JSON parser 可能只留下最後一個值。

### 只引用 zlogger 的完整範例

儲存為 main.go 後執行 `go run .`。程式使用獨立 Instance 寫入暫存檔，
同步及關閉後印出內容，最後清除自己的暫存目錄，不修改全域 logger。

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vincent119/zlogger"
)

type profile struct{ name string }

func (p profile) MarshalLogObject(encoder zlogger.ObjectEncoder) error {
	encoder.AddString("name", p.name)
	return encoder.AddArray("codes", codes{2, 1})
}

type codes []int

func (values codes) MarshalLogArray(encoder zlogger.ArrayEncoder) error {
	for _, value := range values {
		encoder.AppendInt(value)
	}
	return nil
}

var (
	_ zlogger.ObjectMarshaler = profile{}
	_ zlogger.ArrayMarshaler  = codes{}
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() (result error) {
	directory, err := os.MkdirTemp("", "zlogger-structured-fields-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(directory)) }()
	cfg := zlogger.DefaultConfig()
	cfg.Format, cfg.Outputs = "json", []string{"file"}
	cfg.LogPath, cfg.FileName, cfg.AddCaller = directory, "fields.log", false
	instance, err := zlogger.New(cfg)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, instance.Close()) }()
	instance.Logger().Info("structured fields",
		zlogger.Object("profile", profile{"reader"}),
		zlogger.Array("items", codes{3, 1}),
		zlogger.Inline(profile{"inline"}),
		zlogger.Ints("attempts", []int{1, 2}),
		zlogger.Uint8s("numbers", []uint8{65, 66}),
		zlogger.ByteStrings("texts", [][]byte{[]byte("AB")}),
		zlogger.Durations("elapsed", []time.Duration{1500 * time.Millisecond}),
		zlogger.Times("started_at", []time.Time{time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)}),
		zlogger.Namespace("request"),
		zlogger.String("id", "r1"),
	)
	if err := instance.Sync(); err != nil {
		return err
	}
	if err := instance.Close(); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(directory, cfg.FileName))
	if err != nil {
		return err
	}
	_, err = fmt.Print(string(data))
	return err
}
```

輸出包含初始化與應用程式各一筆日誌，ts 會隨執行時間不同。
應用程式日誌的 profile 是巢狀物件，items 保留 `[3,1]` 順序，
name 與 codes 由 Inline 展開至頂層，request.id 位於子物件。
numbers 是 `[65,66]`，texts 是 `["AB"]`；預設 encoder 下，
elapsed 為 `["1.5s"]`，started_at 使用時間字串，不由 helper 固定格式。

## 與遮罩搭配

先建構 `NewRedactingCore`，再綁定 With／WithLazy 或送入 context 欄位。
遮罩關閉時維持原行為，手動 `Redacted` 則一直輸出固定字串。

啟用後按每一層欄位名稱比對，不使用 parent.token 路徑語法。
Reflect、Stringer、Error 會整欄遮罩，並非只處理命中的 key。
新增 helper 仍受相同政策約束：命中的 Object／Array 容器不求值，
命中的 Namespace 抑制整個子樹，Inline 依展開後的欄位檢查。
編碼錯誤、panic、深度或記錄預算超限時整筆拒絕，不寫出部分資料。
opaque map／struct 仍依既有整欄政策處理，不因新增 helper 改成逐欄保留。
marshaler 仍是呼叫端程式碼，包裝不能隔離任意副作用，也不能停止不返回的方法。
完整用法與失敗行為見[安全性](security.md)。
