# 任務文件：SplitOutput 與 Context 效能基線

Status: Complete

## Execution Context

- 意圖：補齊 Context 配置與 SplitOutput 單鎖的可重現 benchmark、兩版工具鏈樣本及 mutex profile
- 本輪授權：使用者已指示 `run task`，依本文件執行 benchmark、量測、DESIGN 與本機驗證；commit、push 與遠端驗收需另行授權
- 非目標：不拆鎖、不加緩衝／非同步、不改 Context 結構、不建立效能 gate、不修改 CI 或 dependency
- 已定決策：所有名稱以 `BenchmarkLogger` 開頭；Context 覆蓋日誌合併與 1／5／20 欄位建構；SplitOutput 使用無狀態記憶體 sink 與真正 `Write` mutex；數據支持時另立 Refactor spec
- 邊界：實作只允許 `benchmark_test.go`、`DESIGN.md`、本 spec，以及 T8 明列的一個歷史 checkbox
- 關鍵檔案：`benchmark_test.go`、`context.go`、`split_output.go`、`DESIGN.md`
- 完成條件：三組 benchmark 可由 `make bench` 發現；兩版 Go 各 10 次樣本；mutex profile 可定位；完整 verify／race 通過；產品碼與公開契約零變更

### Protected Behavior

- SplitOutput level routing、單鎖正確性、rotation、Close、Sync 與 `os.ErrClosed` 不變。
- Context defensive copy、欄位順序、nil context 與 global logger 行為不變。
- 檔案安全、權限 options、Config schema、encoder 與 dependency 不變。
- 既有 `BenchmarkLoggerInfoDisabled`、`BenchmarkLoggerInfoFields` 及 `make bench` 行為不變。
- CI benchmark 仍為 smoke test，不新增或宣稱數值 gate。

### 邊界

#### Allowed Changes

- `benchmark_test.go`
- `DESIGN.md`
- `.specs/2026-07-29-16-26_Chore-split-context-performance-baseline/`
- T8 僅可勾選 `.specs/2026-07-29-10-52_Refactor-config-init-contract/tasks.md` 的「另立一般／SplitOutput benchmark 與鎖競爭評估 spec」

#### Forbidden

- 修改 `context.go`、`split_output.go`、`core.go` 或其他產品 `.go` 檔
- 修改公開 API、Config／ConfigPatch、FileOutputOption、error 或輸出格式
- 修改 Makefile、`.github/`、`.golangci.yml`、go.mod、go.sum 或 Codecov
- 使用真實磁碟、網路、stdout、sleep、換檔 goroutine 或共享 atomic counter 量測 SplitOutput 鎖
- 新增 dependency、benchmark gate、buffer、pool、unsafe、拆鎖或 exported test seam
- 提交 benchmark output、profile、coverage 或其他產物

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 固定缺口與 benchmark 契約 | 無 | Complete | 既有兩案例與 BenchmarkLogger pattern 已確認 |
| T2 新增 Context benchmark | T1 | Complete | 日誌合併及 1／5／20 欄位建構完成 |
| T3 新增 SplitOutput 鎖 benchmark | T1 | Complete | 無狀態 sink、serial／parallel 完成 |
| T4 收集兩版樣本與 mutex profile | T2、T3 | Complete | 同提交、同硬體、各 10 次樣本完成 |
| T5 更新 DESIGN 與分析結論 | T4 | Complete | 已寫入可追溯數據與限制 |
| T6 完整品質與邊界驗證 | T2 至 T5 | Complete | verify、兩版 race、產物檢查通過 |
| T7 遠端 benchmark smoke 驗收 | T6 | Complete | PR #16 的七項 CI 與新增 selectors 全部通過 |
| T8 回填完成狀態與歷史待辦 | T7 | Complete | 規格狀態與明列歷史 checkbox 已回填 |

## 實作任務

- [x] T1 固定現況、selector 與量測契約
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：benchmark、產品碼與其他文件
  - Depends：無
  - Context：記錄目前只有 `BenchmarkLoggerInfoDisabled`、`BenchmarkLoggerInfoFields`；確認新 selectors 尚不存在，`make bench` 使用 `BenchmarkLogger` pattern。固定 payload、欄位組合、子案例命名、Go 版本、count、benchtime 與暫存產物位置。
  - Verify：
    - `rg -n '^func Benchmark|BENCH_PATTERN' benchmark_test.go Makefile`
    - `go test -run=NONE -bench='BenchmarkLogger' -benchmem -benchtime=100x ./...`

- [x] T2 新增 Context 日誌與建構 benchmark
  - Status: Complete
  - Boundary:
    - Allowed Changes：`benchmark_test.go`
    - Forbidden：`context.go`、global logger 產品邏輯、其他測試與文件
  - Depends：T1
  - Context：新增 `BenchmarkLoggerInfoContext` 的 direct／context-only／context-with-fields，以及 `BenchmarkLoggerWithContext` 的 1／5／20 欄位 batch／incremental。logger、固定欄位與 base context 在 timer 外建立；結果避免被消除；global logger 必須 Cleanup 恢復。不得平行執行會寫 global logger 的 benchmark。
  - Verify：
    - `go test -run=NONE -bench='BenchmarkLogger(InfoContext|WithContext)' -benchmem -count=5 ./...`
    - `go test -race -count=1 -run 'Test(Context|WithContext|FromContext|InfoContext)' ./...`

- [x] T3 新增 SplitOutput 串行與並行鎖 benchmark
  - Status: Complete
  - Boundary:
    - Allowed Changes：`benchmark_test.go`
    - Forbidden：`split_output.go`、真實檔案、worker、timer、其他測試與文件
  - Depends：T1
  - Context：新增無狀態 `writeSyncCloser` test sink，直接組裝不含 worker 與資源 ownership 的 SplitOutput。`BenchmarkLoggerSplitOutputWrite` 包含 serial、parallel-same-level、parallel-mixed-level；固定 payload、ReportAllocs、SetBytes。mixed level 使用 worker-local 索引，不使用共享 counter。
  - Verify：
    - `go test -run=NONE -bench='BenchmarkLoggerSplitOutputWrite' -benchmem -count=5 ./...`
    - `go test -race -count=1 -run 'Test(SplitOutput|GetSplitCore)' ./...`
    - `rg -n 'RunParallel|ReportAllocs|SetBytes|BenchmarkLoggerSplitOutputWrite' benchmark_test.go`

- [x] T4 收集 Go 1.25.11／1.26.5 樣本與 mutex profile
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes；暫存目錄中的輸出與 profile
    - Forbidden：repository 內新增量測產物、產品碼或 benchmark 調整
  - Depends：T2、T3
  - Context：在同一提交與同一硬體，以兩版工具鏈各執行 10 次目標 benchmark；使用 CI baseline 已固定的 benchstat 版本比較。另只對 SplitOutput parallel 案例產生 mutex profile，以 top 確認競爭位置。記錄命令、環境、觀察與雜訊限制，不設定 gate。
  - Verify：
    - Go 1.25.11／1.26.5：`go test -run=NONE -bench='BenchmarkLogger(InfoContext|WithContext|SplitOutputWrite)' -benchmem -count=10 ./...`
    - `go test -run=NONE -bench='BenchmarkLoggerSplitOutputWrite/parallel' -benchtime=2s -mutexprofile=<temp>/split-output-mutex.out ./...`
    - `go tool pprof -top <temp>/split-output-mutex.out`
    - `git status --short` 不含輸出或 profile

- [x] T5 更新 DESIGN 效能基線與決策結論
  - Status: Complete
  - Boundary:
    - Allowed Changes：`DESIGN.md`、本 tasks Implementation Notes
    - Forbidden：README、產品碼、CI、Makefile 與未實作承諾
  - Depends：T4
  - Context：記錄新增案例、可重現命令、兩版工具鏈觀察、mutex profile、環境限制及是否需要後續 Refactor spec。不得把 hosted runner 或單機數字宣稱為跨環境 SLA，不得承諾拆鎖或零配置。
  - Verify：
    - `rg -n 'BenchmarkLoggerInfoContext|BenchmarkLoggerWithContext|BenchmarkLoggerSplitOutputWrite|mutex|benchstat|1.25.11|1.26.5' DESIGN.md benchmark_test.go`
    - DESIGN 數值可追溯至 T4 實際輸出

- [x] T6 完整品質、相容與邊界驗證
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過驗證擴張產品或 workflow 變更
  - Depends：T2 至 T5
  - Context：執行兩版完整 race、目標 benchmark、make verify、格式、lint、coverage、diff 與產物檢查。確認產品碼、公開 API、Makefile、CI 及 dependency 無差異。
  - Verify：
    - Go 1.25.11：`go test -race -count=1 ./...`
    - Go 1.26.5：`go test -race -count=1 ./...`
    - `make verify`
    - `git diff --check`
    - `git diff --name-only` 只含 Allowed Changes
    - `git status --short` 不含 benchmark、profile 或 coverage 產物

- [x] T7 遠端 benchmark smoke 與跨平台驗收
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過 CI 修改 workflow、benchmark 語意、skip 或門檻
  - Depends：T6
  - Context：經使用者另行授權 commit／push 後，確認既有七項 CI 全部通過；benchmark job 必須列出新增案例，macOS／Windows 與兩版 race 不得受全域 logger 或 test-only sink 影響。
  - Verify：`gh pr checks` 或 `gh run view` 顯示七個 jobs pass，benchmark log 包含三個新 selector

- [x] T8 回填完成狀態與歷史待辦
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 requirements／tasks；Execution Context 明列的一個歷史 checkbox
    - Forbidden：其他 spec、產品碼或文件內容重寫
  - Depends：T7
  - Context：遠端 green 後將本 requirements／tasks 標記 Complete，附 PR、merge commit、run／job 證據；只勾選 Config 初始化規格已完成的 benchmark／鎖競爭後續項目。
  - Verify：文件狀態、checkbox 與 GitHub 證據一致

## 驗證任務

- [x] V1 benchmark 案例完整性
  - 三個新 selectors 可由 `make bench` 發現
  - Context 有 direct／context-only／context-with-fields 及 1／5／20 batch／incremental
  - SplitOutput 有 serial／parallel-same-level／parallel-mixed-level
  - 所有案例有 `ReportAllocs`，SplitOutput 有 `SetBytes`

- [x] V2 量測隔離與可重現性
  - logger、固定資料與 Context 在 timer 外準備
  - SplitOutput sink 無狀態、無額外鎖、無磁碟／worker／sleep
  - 固定 Go 版本、count、selector、payload 與同機比較條件
  - benchmark 與 profile 產物不進 Git

- [x] V3 正確性與安全回歸
  - SplitOutput routing、rotation、Close、Sync、權限與 containment 測試通過
  - Context defensive copy、欄位順序、nil 與並行測試通過
  - Go 1.25.11／1.26.5 完整 race 通過

- [x] V4 品質與邊界
  - `make verify` 通過
  - fmt、vet、lint、coverage 與 benchmark smoke 通過
  - 產品碼、公開 API、Makefile、CI、dependency 無差異
  - DESIGN 與 spec 不過度承諾效能或後續重構

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 `Execution Context`
2. 目前未完成 task
3. `Protected Behavior`
4. `Implementation Notes`

不得預設掃描整個 `.specs` 目錄。定位命令：

```bash
rg -n '^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:' .specs/2026-07-29-16-26_Chore-split-context-performance-baseline
```

## Implementation Notes

- 2026-07-29：從同步至 PR #15 merge commit `aa2afc4` 的 main 建立分支 `chore/split-context-performance-baseline`。
- 2026-07-29：現況只有 `BenchmarkLoggerInfoDisabled` 與 `BenchmarkLoggerInfoFields`；兩者使用 discard sink、ReportAllocs，並由 `BENCH_PATTERN=BenchmarkLogger` 納入 `make bench`。
- 2026-07-29：Context 每次合併既有欄位會建立新 slice；逐次 WithContext 會重複複製，但 defensive copy 是已完成的安全契約，本 spec 只量測不移除。
- 2026-07-29：SplitOutput 的 Write 以單一 mutex 包住 level routing 與 writer.Write；lifecycle、關閉後錯誤與 rotation 競態已修正，本 spec 不重寫。
- 2026-07-29：requirements、design、tasks 已建立；依 SDD 流程停在規劃階段，尚未修改 benchmark、產品碼、DESIGN、歷史 checkbox 或執行量測。
- 2026-07-29：使用者指示 `run task`，requirements 與 tasks 狀態改為 InProgress，從 T1 現況基線開始執行；本批不含 commit、push 或遠端驗收。
- 2026-07-29：T1 確認目前只有兩個 `BenchmarkLogger` 案例，Makefile pattern 可發現並執行；Go 1.26.5 darwin/arm64、Apple M1 的 100 次 smoke 為 disabled 127.1 ns/op、0 B/op、0 allocs/op，fields 2693 ns/op、91 B/op、2 allocs/op。首次因預設 Go cache 權限失敗，改用 `/private/tmp/zlogger-go-cache` 後通過，未修改持久環境。
- 2026-07-29：T2 新增 `BenchmarkLoggerInfoContext` 與 `BenchmarkLoggerWithContext`；logger、固定欄位與 base context 均在 timer 外準備，global logger 以 Cleanup 恢復，1／5／20 欄位皆有 batch／incremental 子案例。
- 2026-07-29：T3 新增無狀態 `benchmarkWriteSyncCloser` 與 `BenchmarkLoggerSplitOutputWrite`；serial、parallel same level、parallel mixed level 均走真正 `SplitOutput.Write`，使用固定 77-byte payload、ReportAllocs 與 SetBytes，不建立檔案或 worker。初版共用驗證 helper 會在熱路徑呼叫 `b.Helper()`，審閱後改為各案例直接檢查結果，避免污染量測。
- 2026-07-29：T2／T3 的 100 次 smoke 與 Context／SplitOutput 目標 race 通過。Go 1.26.5 smoke 顯示 20 欄位 batch 為 3 allocs/op、incremental 為 60 allocs/op；SplitOutput 三案例均為 0 allocs/op。
- 2026-07-29：T4 在 Apple M1 darwin/arm64、同一工作樹，以 Go 1.25.11／1.26.5 各執行 10 次固定 selectors；輸出存於 `/private/tmp/zlogger-performance-baseline.UZQ0ik`。首次安裝固定 benchstat 因 sandbox DNS 失敗，經授權後安裝 CI 基線 commit `82a0b07e230d76fa1b3036c383d7a98172f87334` 至 `/private/tmp/zlogger-tools`，未修改 go.mod／go.sum。
- 2026-07-29：benchstat 顯示兩版 B/op、allocs/op 完全一致；具統計顯著的 sec/op 變化皆低於 10%，無超過 10% 退化證據。Go 1.26.5 的 20 欄位 batch／incremental 中位數為 201.8 ns／3.351 µs、1.445 KiB／15.59 KiB、3／60 allocs。
- 2026-07-29：2 秒 SplitOutput parallel mutex profile 顯示 `(*SplitOutput).Write` 累積涵蓋 96.59% mutex delay，`sync.(*Mutex).Unlock` 為 86.37% flat delay；此結果只證明無狀態 sink 合成案例的單鎖競爭，是否重構仍需實際服務 profile。
- 2026-07-29：T5 更新 DESIGN 的配置成本、Context／SplitOutput 基線、可重現命令與限制；保留單鎖以維持 writer／rotation ownership，只有實際服務 profile 亦顯著時才另立 Refactor spec。
- 2026-07-29：T6 的 Go 1.25.11／1.26.5 完整 race 均通過；`make verify GOLANGCI_LINT=/private/tmp/zlogger-tools/golangci-lint` 通過 fmt-check、vet、golangci-lint v2.12.2、race、92.5% coverage gate 與全部 `BenchmarkLogger` smoke。linter 因 sandbox 無法寫入預設 cache 出現 warning，但結果為 0 issues。
- 2026-07-29：`make clean` 已移除 coverage 產物；`git diff --check` 通過，差異只有 `benchmark_test.go`、`DESIGN.md` 與本 spec 三份文件。未修改產品碼、公開 API、Makefile、CI、dependency 或歷史 checkbox，benchmark／profile 輸出只存在 `/private/tmp`。
- 2026-07-29：PR #16 已合併為 `18c23c9`；GitHub Actions run `30437300497` 的 macOS 15、Windows 2025、Go 1.25.11／1.26.5 race、靜態與格式、coverage／Codecov、benchmark 共七項工作全部通過，完成 T7。
- 2026-07-29：遠端 benchmark job `90527941891` 已列出 `BenchmarkLoggerInfoContext`、`BenchmarkLoggerWithContext`、`BenchmarkLoggerSplitOutputWrite` 的全部子案例；確認既有 `BenchmarkLogger` pattern 能在 CI 執行新基線。
- 2026-07-29：完成 T8；本 spec 狀態已回填為 Complete，並依明列邊界勾選 Config 初始化規格的「另立一般／SplitOutput benchmark 與鎖競爭評估 spec」。

## 驗證結果摘要

- 新行為驗證：通過；三組新 selectors 的 100 次 smoke、各 5 次目標執行、兩版各 10 次樣本與 mutex profile 完成
- 回歸驗證：通過；Go 1.25.11／1.26.5 完整 race 與 `make verify` 通過
- 文件一致性：已確認 DESIGN、benchmark selectors、requirements、design 與 tasks 一致
- 剩餘風險：無阻擋風險；合成 mutex profile 不代表真實磁碟工作負載，產品重構仍需服務 profile 證據

## 後續改善

- [ ] 若 mutex profile 與樣本顯示顯著競爭，另立 SplitOutput writer ownership／拆鎖 Refactor spec
- [ ] 若 Context 配置成本在實際服務 profile 中占比顯著，另立不可變欄位結構或預綁 logger Refactor spec
- [ ] 累積足夠穩定的專用 runner 歷史樣本後，再評估效能回歸門檻
