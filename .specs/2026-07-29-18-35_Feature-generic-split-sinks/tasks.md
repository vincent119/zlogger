# 任務文件：通用分級 Sink

Status: InProgress

## Execution Context

- 意圖：新增公開 `SplitSinks`／`NewSplitCore`，讓外部 `zapcore.WriteSyncer` 可使用既有
  DEBUG／INFO、WARN、ERROR 以上分級路由
- 本輪授權：使用者已指示執行及交付 tasks；T1 至 T6 已完成，本輪可 commit、push 並
  準備 PR 內容；受限 Token 不建立 PR，T7 遠端 CI 與 T8 回填仍待後續驗收
- 非目標：不加入 timberjack dependency／Adapter，不公開 Factory，不修改每日換檔、
  `SplitOutput` lifecycle、安全開檔、Config 或 CI
- 已定決策：新 API 接收 `zapcore.Encoder` 與三個 sinks；每路 clone encoder；sink ownership
  保留於呼叫端；nil 輸入以 `ErrInvalidSplitCore` 分類；既有 GetSplitCore 共用 core helper
- 邊界：規劃階段只允許本 spec；進入實作後依各 task 的 Allowed Changes 執行
- 關鍵檔案：`split_output.go`、`split_output_test.go`、example、`README.md`、`DESIGN.md`
- 完成條件：新 API 路由、validation、Sync、ownership 與 example 驗收完成；既有
  SplitOutput 回歸及完整 verify 通過；go.mod／go.sum 無變更

### Protected Behavior

- `NewSplitOutput`／`NewSplitOutputWithOptions` 的檔名、每日換檔、權限與錯誤不變。
- `GetSplitCore`／`GetSplitCoreWithOptions` 的簽章、JSON 編碼、routing 與 cleanup 不變。
- DEBUG／INFO、WARN、ERROR 以上的標準 level mapping 不變且不得重複寫入。
- `SplitOutput.Close` 的 worker 停止、冪等、並行與 `os.ErrClosed` 契約不變。
- `os.Root` containment、leaf validation、symlink、permission 與 umask 行為不變。
- Config、全域 logger、一般 file output、Context、encoder compatibility、benchmark 與 CI 不變。
- 核心 dependency 仍只有既有 zap；不得加入 timberjack。

### 邊界

#### Allowed Changes

- `.specs/2026-07-29-18-35_Feature-generic-split-sinks/`
- 實作 task 明列的 `split_output.go`、`split_output_test.go`
- 一個既有或新增的 `*_example_test.go`
- `README.md`
- `DESIGN.md`

#### Forbidden

- `go.mod`、`go.sum`
- `config.go`、`core.go`、`context.go`、`encoder.go`、file security／permission 產品碼
- Makefile、`.github/`、`.golangci.yml`、Codecov 或 release 設定
- 新增 dependency、Adapter、Factory、rotation options、buffer、goroutine 或 runtime sink replacement
- 修改現有公開 API 簽章、檔名格式、預設權限、錯誤分類或 cleanup ownership
- 處理未列入契約的 typed nil、sink identity 去重或非標準 level 保證
- 修改既有未追蹤檔案或與本功能無關的使用者變更

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 固定公開 API 與現況基線 | 無 | Complete | API、zap 契約與既有測試基線已確認 |
| T2 以 TDD 建立新 API 驗收測試 | T1 | Complete | 缺少新 API 的預期編譯紅燈已確認 |
| T3 實作通用分級 core | T2 | Complete | 公開 API、驗證與 encoder clone 完成 |
| T4 讓既有 GetSplitCore 共用 helper | T3 | Complete | 原 routing 與 cleanup 回歸通過 |
| T5 新增可編譯 example 與文件 | T3、T4 | Complete | example 與分級 rotation 文件通過 |
| T6 完整品質與相容驗證 | T2 至 T5 | Complete | 92.7% coverage、race、lint 與 benchmark 通過 |
| T7 遠端跨平台驗收 | T6 | Planned | 需另行取得 push／PR 授權 |
| T8 回填 spec 完成狀態 | T7 | Planned | 僅在遠端 green 後完成 |

## 實作任務

- [x] T1 固定公開 API、ownership 與現況基線
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks 的 Implementation Notes
    - Forbidden：產品碼、測試、README、DESIGN 與其他 spec
  - Depends：無
  - Context：確認 `SplitSinks`、`NewSplitCore`、`ErrInvalidSplitCore` 命名；確認
    `zapcore.Encoder.Clone` 與 `zapcore.WriteSyncer` 契約；記錄既有七個標準 level routing、
    GetSplitCore Sync 及 cleanup baseline。若公開 API 必須變更，先更新 requirements／design，
    不得直接在實作中偏移。
  - Verify：
    - `go test -count=1 -run 'Test(GetSplitCore|SplitOutput)' ./...`
    - `go test -race -count=1 -run 'Test(GetSplitCoreRoutesLevels|SplitOutputCloseStopsRotation)' ./...`
    - `go doc go.uber.org/zap/zapcore.Encoder`
    - `go doc go.uber.org/zap/zapcore.WriteSyncer`

- [x] T2 以 TDD 建立新 API 驗收測試
  - Status: Complete
  - Boundary:
    - Allowed Changes：`split_output_test.go`
    - Forbidden：產品碼、example、README、DESIGN 與 dependency
  - Depends：T1
  - Context：新增 thread-safe 記憶體 sink、Sync 計數及 Close tracking；table-driven 覆蓋
    encoder／info／warn／error nil，以及 DEBUG 至 FATAL 七級。測試先因 API 尚不存在而失敗，
    記錄預期紅燈；不得以修改測試規避既有 routing。
  - Verify：
    - `go test -count=1 -run 'TestNewSplitCore' ./...` 預期先因缺少 API 失敗
    - `rg -n 'TestNewSplitCore(RoutesLevels|RejectsInvalidInputs|SyncsSinks|DoesNotCloseSinks)' split_output_test.go`
    - `git diff --check`

- [x] T3 實作公開通用分級 core
  - Status: Complete
  - Boundary:
    - Allowed Changes：`split_output.go`、本 tasks Implementation Notes
    - Forbidden：既有 file opener、clock、rotation、Close、Config、其他產品檔與文件
  - Depends：T2
  - Context：新增 sentinel、`SplitSinks`、`NewSplitCore`、輸入驗證及 package-private
    `buildSplitCore`。每路使用 encoder clone，路由只涵蓋標準契約；不建立或關閉 sink，
    不加入反射式 typed nil 或 identity 判斷。
  - Verify：
    - `gofmt -w split_output.go split_output_test.go`
    - `go test -count=1 -run 'TestNewSplitCore' ./...`
    - `go test -race -count=1 -run 'TestNewSplitCore' ./...`
    - `go vet ./...`
    - `git diff --check`

- [x] T4 讓既有 GetSplitCore 共用 routing helper
  - Status: Complete
  - Boundary:
    - Allowed Changes：`split_output.go`、`split_output_test.go`、本 tasks Implementation Notes
    - Forbidden：`SplitOutput.Write`、openers、rotation、Close、公開函式簽章與其他產品檔
  - Depends：T3
  - Context：將既有 JSON encoder 與三個 wrappers 組成 `SplitSinks` 後交給共用 helper；
    cleanup 仍只關閉自有 `SplitOutput`。刪除重複的三組 core 組裝，但不得改寫 direct
    `SplitOutput.Write` 路由。
  - Verify：
    - `go test -count=1 -run 'Test(GetSplitCore|SplitOutputSync)' ./...`
    - `go test -race -count=1 -run 'Test(GetSplitCore|SplitOutput)' ./...`
    - `go test -count=20 -run 'Test(NewSplitCoreRoutesLevels|GetSplitCoreRoutesLevels)' ./...`
    - `git diff --check`

- [x] T5 新增公開 example、README 與 DESIGN
  - Status: Complete
  - Boundary:
    - Allowed Changes：一個 `*_example_test.go`、`README.md`、`DESIGN.md`、本 tasks
      Implementation Notes
    - Forbidden：產品碼、非本功能文件、dependency、CI 與無法編譯的外部範例測試
  - Depends：T3、T4
  - Context：example 使用專案現有 dependency 與 deterministic sink，展示
    `SplitSinks`、`NewSplitCore`、Sync 及呼叫端 ownership。README 補 timberjack 三 sink
    文件片段，但不匯入 example test；DESIGN 記錄路由／rotation 分層、不採 Factory、
    不接管 Close 與三個獨立 sink 的要求。
  - Verify：
    - `go test -count=1 -run 'ExampleNewSplitCore' ./...`
    - `rg -n 'SplitSinks|NewSplitCore|ownership|timberjack|WriteSyncerFactory' README.md DESIGN.md`
    - `git diff -- go.mod go.sum`
    - `git diff --check`

- [x] T6 完整品質、相容與邊界驗證
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes；`make clean` 產物清理
    - Forbidden：為通過驗證擴張功能、修改 CI／門檻或新增 dependency
  - Depends：T2 至 T5
  - Context：執行完整 race、連續 routing、make verify、API 文件、diff、coverage 及
    dependency 檢查。確認既有 SplitOutput lifecycle、安全、權限與效能保護行為未變。
  - Verify：
    - `go test -race -count=1 ./...`
    - `go test -count=20 -run 'Test(NewSplitCoreRoutesLevels|GetSplitCoreRoutesLevels|SplitOutputCloseStopsRotation)' ./...`
    - `make verify`
    - `go doc . SplitSinks`
    - `go doc . NewSplitCore`
    - `git diff --check`
    - `git diff -- go.mod go.sum`
    - `git diff --name-only` 僅包含 Allowed Changes
    - `git status --short` 不新增 coverage、profile 或其他產物

- [ ] T7 遠端 CI 與跨平台驗收
  - Status: Planned
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：未經授權 commit／push／PR；為通過 CI 修改 workflow、skip 或測試語意
  - Depends：T6
  - Context：使用者另行授權 commit／push 並建立 PR 後，確認 Go 1.25.11／1.26.5、
    macOS、Windows、lint、coverage、benchmark 既有 jobs 全數通過。新 API 不應依賴
    Unix 檔案語意或 timberjack。
  - Verify：`gh pr checks` 或 `gh run view` 顯示所有既有必要 jobs pass

- [ ] T8 回填規格完成狀態與證據
  - Status: Planned
  - Boundary:
    - Allowed Changes：本 spec 三份文件
    - Forbidden：產品碼、README、DESIGN、其他 spec 與 release 檔
  - Depends：T7
  - Context：只有本機驗證及遠端 CI 都通過後，才能把 requirements／design／tasks
    標記 Complete，勾選品質清單並記錄 commit、PR、run／job 證據。
  - Verify：狀態、checkbox、Implementation Notes 與實際 GitHub 證據一致

## 驗證任務

- [x] V1 公開 API 與文件
  - `SplitSinks`、`NewSplitCore`、`ErrInvalidSplitCore` 具繁體中文 Go doc
  - routing、encoder clone、nil、ownership 與三個獨立 sink 契約明確
  - README example 與實際函式簽章一致

- [x] V2 新行為正確性
  - 七個標準 level 各自只寫入一個預期 sink
  - encoder／三個 sink nil 時不回傳部分 core
  - `core.Sync()` 委派三個配置欄位
  - zlogger 不呼叫外部 sink `Close`

- [x] V3 既有行為回歸
  - GetSplitCore JSON、routing、Sync、cleanup 與 options 不變
  - SplitOutput rotation、Close、權限、containment 與錯誤契約通過
  - 完整 race 與連續 20 次 routing／lifecycle 測試通過

- [x] V4 依賴與品質邊界
  - go.mod／go.sum 無差異，沒有新增 timberjack
  - make verify、coverage gate、lint、vet、fmt 與 diff check 通過
  - 變更檔案只落在 Allowed Changes
  - 未追蹤使用者檔案保持不動，沒有測試或 coverage 產物進 Git

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 `Execution Context`
2. 目前未完成 task
3. `Protected Behavior`
4. `Implementation Notes`

定位命令：

```bash
rg -n '^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:' \
  .specs/2026-07-29-18-35_Feature-generic-split-sinks
```

## Implementation Notes

- 2026-07-29：確認現有 `SplitOutput` 已有 package-private opener 與 file set，但沒有公開
  sink 注入 API；公開 `WriteSyncerFactory` 不是本功能所需。
- 2026-07-29：決定先提供 dependency-neutral 的 `SplitSinks`／`NewSplitCore`，不直接加入
  timberjack。timberjack Adapter 是否建立，待此 API 發布後依真實使用需求另立 spec。
- 2026-07-29：決定外部 sink ownership 永遠屬呼叫端；新 API 不回傳 cleanup，避免
  `zapcore.WriteSyncer` 缺少 Close 契約所造成的 double close。
- 2026-07-29：requirements、design、tasks 已建立；依 SDD 流程停在規劃階段，尚未修改
  產品碼、測試、README、DESIGN、dependency 或 CI。
- 2026-07-31：使用者指示開始執行 tasks；本輪授權更新為 T1 至 T6，本機完成後停在
  commit／push／PR 前，T7、T8 保持 Planned。
- 2026-07-31：T1 確認 zap 1.28.0 的 `Encoder.Clone` 明確隔離累積欄位，
  `WriteSyncer` 只包含 Write／Sync、不包含 Close；因此既定 encoder clone 與外部
  ownership 契約成立。既有 GetSplitCore／SplitOutput 單元與目標 race 基線通過。
- 2026-07-31：T2 新增四組 `NewSplitCore` 驗收測試與 thread-safe 記憶體 sink；使用
  `/private/tmp/zlogger-go-cache` 避開 sandbox 的預設 Go cache 權限限制後，確認缺少
  `NewSplitCore`、`SplitSinks`、`ErrInvalidSplitCore` 的預期編譯紅燈。
- 2026-07-31：T3 新增公開 sentinel、三路 sink 型別、輸入驗證與 `buildSplitCore`；三個
  core 各自 clone encoder，不取得外部 sink Close ownership。目標單元、race 與 vet 通過。
- 2026-07-31：T4 將 `GetSplitCoreWithOptions` 的三組重複 level enabler 改為共用
  `buildSplitCore`；既有 JSON encoder、wrappers、cleanup 與 direct SplitOutput routing
  未變。目標單元／race 及新舊 routing 連續 20 次通過。
- 2026-07-31：T5 新增外部 package 的 `ExampleNewSplitCore`，以 deterministic JSON
  驗證 info／warn／error 三路並呈現呼叫端 Close ownership。README 與 DESIGN 新增
  通用 sink、timberjack 三 sink、單一 rotation owner 及不採 Factory 的說明；同步修正
  timberjack 現行壓縮欄位為 `Compression`。example test 與文件／dependency diff 通過。
- 2026-07-31：T6 的新舊 routing／lifecycle 連續 20 次通過，公開 API `go doc` 與契約
  一致。第一次 `make verify` 因 sandbox 禁止寫入使用者 golangci-lint cache 而在 0 issues
  後退出；改用 `/private/tmp/zlogger-golangci-cache` 重跑後，fmt、vet、golangci-lint
  v2.12.2、完整 race、92.7% atomic coverage 與全部 BenchmarkLogger smoke 通過。
- 2026-07-31：`make clean` 已移除 coverage 產物；`git diff --check` 與 dependency diff
  通過，go.mod／go.sum、Config、CI、Makefile 及其他 Forbidden 檔案無變更。T1 至 T6
  完成，requirements／design／tasks 保持 InProgress，等待 T7 遠端 CI 與 T8 回填。
- 2026-07-31：使用者授權進入交付階段；本輪提交並推送 Feature branch，由使用者透過
  GitHub 網頁建立 PR，建立後再執行 T7 遠端 CI 驗收。

## 後續改善

- [ ] 收集公開 API 使用回饋，確認是否需要獨立 timberjack Adapter spec
- [ ] 只有實際需求證明需要 runtime sink replacement 時，才評估 Factory spec
- [ ] 若外部使用者需要統一 Close 聚合，另立 ownership-aware resource group spec
