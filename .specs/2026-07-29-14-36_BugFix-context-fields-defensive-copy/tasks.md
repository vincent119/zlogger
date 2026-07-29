# 任務文件：Context fields 邊界防禦性複製

Status: InProgress

## Execution Context

- 意圖：隔離 `WithContext` 輸入、context 內部儲存與 `FromContext` 輸出的 `[]Field` 底層陣列，消除 API aliasing 與相關 data race
- 本輪授權：只建立 requirements、design、tasks；使用者再次明確指示後才進入產品碼與測試實作
- 非目標：不深拷貝 `Field.Interface`，不修改 Context API、key、全域 logger、輸出 core、encoder、SQL、README.en 或 dependency
- 已定決策：輸入與公開輸出淺層複製；新增 package-private `contextFields`；內部 merge 使用私有 accessor 避免雙重配置
- 邊界：實作限於 `context.go`、`context_test.go`、`DESIGN.md`、本 spec，以及結案時前置安全 spec 的單一 Context 待辦
- 關鍵檔案：`context.go`、`context_test.go`、`DESIGN.md`
- 完成條件：deterministic mutation tests、race、目標測試 20 次、`make verify`、coverage >= 90%、遠端七項 CI 與文件回填全部通過

### Protected Behavior

- 公開 Context API 簽章、context key 與欄位 key 不變
- nil context、零 fields、空 helper value 的既有行為不變
- 欄位累積順序維持既有在前、新增在後
- Context 日誌 level、欄位內容與 nil global logger no-op 不變
- `Field.Interface` 內部參照不宣稱 deep copy 或 thread-safe
- 不修改 file output、SplitOutput、rotation、encoder 或 SQL

### 邊界

#### Allowed Changes

- `context.go`
- `context_test.go`
- `DESIGN.md` 的 Context ownership 說明
- `.specs/2026-07-29-14-36_BugFix-context-fields-defensive-copy/`
- 遠端驗收後，`.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/tasks.md` 僅更新 Context defensive copy 後續項目

#### Forbidden

- `core.go`、`encoder.go`、`split_output.go`、file security 與其他產品模組
- README、README.en、公開 API、context key 或 field helper 語意
- `go.mod`、`go.sum`、CI、Makefile、lint 或 coverage 設定
- 外部 dependency、deep-copy library、reflection-based clone
- sleep、retry、全域 test hook 或整組平台 skip

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 建立 ownership TDD Red tests | 無 | Complete | Red 證實首次輸入與公開輸出 aliasing |
| T2 實作 Context slice ownership | T1 | Complete | 私有 accessor＋邊界淺層複製 |
| T3 更新 Context 設計契約 | T2 | Complete | 文件明確限定淺層 copy 與 nested reference 責任 |
| T4 本機完整驗證與邊界檢查 | T2、T3 | Complete | 兩版 race、20 次、verify、92.7% coverage |
| T5 遠端跨平台驗收 | T4 | Planned | 七項既有 CI 必須全綠 |
| T6 回填完成狀態 | T5 | Planned | 本 spec 與前置待辦結案 |

## 實作任務

- [x] T1 建立 ownership TDD Red tests
  - Status: Complete
  - Boundary:
    - Allowed Changes：`context_test.go`、本 tasks Implementation Notes
    - Forbidden：產品實作、sleep、retry、只靠 race detector 的非 deterministic 斷言
  - Depends：無
  - Context：現況第一次寫入直接保存輸入 slice，`FromContext` 直接回傳內部 slice；兩條路徑都要先有可重現 Red。
  - Verify：
    - `go test -count=1 -run 'Test(WithContextCopiesInputFields|FromContextReturnsDefensiveCopy)$' ./...` 預期在修正前失敗
    - 子測試需涵蓋 first batch 與 appended batch
    - 記錄 Red 原因，不將預期失敗測試留在最終 commit 狀態

- [x] T2 實作 Context slice ownership
  - Status: Complete
  - Boundary:
    - Allowed Changes：`context.go`、`context_test.go`、本 tasks Implementation Notes
    - Forbidden：公開簽章、deep copy、Boundary 外產品碼
  - Depends：T1
  - Context：新增私有 `contextFields`；`WithContext` 統一配置 owned slice；`FromContext` 使用 `slices.Clone`；`mergeContextFields` 直接讀私有 slice。
  - Verify：
    - `go test -race -count=1 -run 'Test(WithContextCopiesInputFields|FromContextReturnsDefensiveCopy|ContextFieldsDefensiveCopyConcurrentAccess|WithContext_|FromContext_|MergeContextFields_|ContextLogFunctions|MultipleContextFields)' ./...`
    - `go test -count=20 -run 'Test(WithContextCopiesInputFields|FromContextReturnsDefensiveCopy|ContextFieldsDefensiveCopyConcurrentAccess|WithContext_AddFields|MergeContextFields_MergeFields|ContextLogFunctions|MultipleContextFields)' ./...`
    - `rg -n 'contextFields|FromContext|WithContext|mergeContextFields' context.go context_test.go`

- [x] T3 更新 Context 設計契約
  - Status: Complete
  - Boundary:
    - Allowed Changes：`DESIGN.md` 的 Context 章節、`context.go` 相關 godoc、本 tasks Implementation Notes
    - Forbidden：README、README.en、其他設計章節或公開 API
  - Depends：T2
  - Context：文件需說明輸入與輸出 slice ownership、淺層限制、nested reference 非 thread-safe，以及內部 accessor 不得外洩。
  - Verify：
    - `rg -n '防禦性複製|淺層|ownership|Interface' DESIGN.md context.go`
    - 文件不出現 deep copy 或 nested value 已隔離的過度承諾

- [x] T4 本機完整驗證與邊界檢查
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過檢查修改 CI、lint、coverage、dependency 或 Boundary 外檔案
  - Depends：T2、T3
  - Context：驗證新 ownership、不變行為、效能 smoke 與差異範圍；不得以 skip 掩蓋失敗。
  - Verify：
    - `gofmt -w context.go context_test.go`
    - `go test -race -count=1 ./...`
    - 目標 Context selectors `-count=20`
    - `make verify`
    - coverage >= 90%
    - `git diff --stat`
    - `git diff --check`

- [ ] T5 遠端跨平台驗收
  - Status: Planned
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：workflow、skip、sleep、平台專用繞過
  - Depends：T4
  - Context：commit／push 需使用者另行授權；PR 建立後確認既有七項 CI，包括兩版 race、macOS、Windows、lint、coverage 與 benchmark。
  - Verify：`gh pr checks` 或 `gh run view` 顯示七項 jobs 全部 pass

- [ ] T6 回填完成狀態
  - Status: Planned
  - Boundary:
    - Allowed Changes：本 spec；前置安全 spec 只更新 Context defensive copy 待辦
    - Forbidden：其他後續待辦、產品碼或工具設定
  - Depends：T5
  - Context：遠端 green 後將 requirements、tasks 與來源待辦標記 Complete，附 PR、merge commit 與 run 證據。
  - Verify：文件狀態、checkbox 與 GitHub 結果一致

## 驗證任務

- [x] V1 輸入 ownership
  - 首次與追加 fields 都不與 context 共享底層陣列
  - parent 與 child context 不互相污染

- [x] V2 輸出 ownership
  - `FromContext` 回傳值可修改而不污染 context
  - 多次 `FromContext` 的結果互相獨立

- [x] V3 並行與相容性
  - ownership race test 通過
  - nil、空值、順序、helper key 與日誌 level 不變
  - nested reference 明確維持呼叫端責任

- [x] V4 品質與邊界
  - gofmt、vet、lint、race、coverage、benchmark 通過
  - 目標測試連續 20 次通過
  - 差異只包含 Allowed Changes
  - 無 dependency、CI、Makefile、README 或其他產品碼變更
  - `git diff --check` 通過

- [ ] V5 遠端驗收
  - Go 1.25.11／1.26.5 race 通過
  - macOS 15／Windows 2025 通過
  - 靜態分析、coverage gate、benchmark smoke 通過

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 `Execution Context`
2. 目前未完成 task
3. `Protected Behavior`
4. `Implementation Notes`

不得預設掃描整個 `.specs` 目錄。若文件很大，先用標題與關鍵字定位：

```bash
rg -n "^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:" .specs/2026-07-29-14-36_BugFix-context-fields-defensive-copy
```

## Implementation Notes

- 2026-07-29：從已同步 PR #9 的 `main` 建立 `bugfix/context-fields-defensive-copy`。
- 2026-07-29：確認 `WithContext` 首次加入欄位時直接保存呼叫端 slice；`FromContext` 直接回傳內部 slice，兩者均存在可重現 aliasing。
- 2026-07-29：決定採私有 `contextFields` accessor、輸入 owned slice 與公開輸出 clone；內部 merge 不呼叫公開 `FromContext`，避免雙重配置。
- 2026-07-29：防禦範圍只到 `zap.Field` 值的淺層複製；`Field.Interface` 內 map、slice、pointer 等可變資料仍由呼叫端負責同步。
- 2026-07-29：依使用者要求只完成 SDD 文件，尚未修改 `context.go`、`context_test.go` 或 `DESIGN.md`。
- 2026-07-29：T1 新增 table-driven input mutation 與公開 output mutation tests；修正前 Red 中 `first_batch` 讀到 `mutated/changed`，`FromContext` 同時讀到被修改內容且共享底層陣列。`appended_batch` 因既有合併配置而通過。
- 2026-07-29：T2 新增私有 `contextFields`、所有非空 `WithContext` 寫入的 owned slice，以及 `FromContext` 的 `slices.Clone`；`mergeContextFields` 改讀私有 accessor，避免公開 clone 後再次複製。
- 2026-07-29：ownership 目標 race 通過，deterministic、並行與既有 Context selectors 合併執行 20 次通過；輸入、輸出及 context 內部 slice 已互相隔離。
- 2026-07-29：T3 更新 Context API godoc 與 DESIGN ownership 契約，明確說明輸入／輸出防禦性複製為淺層，`Field.Interface` 內參照型資料仍由呼叫端同步。
- 2026-07-29：T4 的 Go 1.26.5 與 Go 1.25.11 完整 race 均通過；Go 1.25.11 toolchain 以官方 checksum database 驗證後下載，本次未修改持久環境設定。
- 2026-07-29：第一次 `make verify` 因本機 golangci-lint v2.12.1 與專案要求 v2.12.2 不符而在 lint 前停止；未降低或修改版本門檻，改將 v2.12.2 安裝至 `/private/tmp/zlogger-tools` 後重跑。
- 2026-07-29：`make verify GOLANGCI_LINT=/private/tmp/zlogger-tools/golangci-lint` 通過 fmt-check、vet、lint 0 issues、race、92.7% coverage gate 與 benchmark smoke；`git diff --check` 通過，coverage 產物已由 `make clean` 移除。
- 2026-07-29：差異只包含 `context.go`、`context_test.go`、`DESIGN.md` 與本 spec；私有 `contextFields` 僅有 package 內三個唯讀呼叫點，未修改 dependency、CI、Makefile、README 或 Boundary 外產品碼。T5 保留等待 commit／push 後的遠端驗收。

## 驗證結果摘要

- 新行為驗證：通過；deterministic input/output mutation、ownership race 與目標測試 20 次均通過
- 回歸驗證：通過；Go 1.25.11／1.26.5 完整 race 與 `make verify` 通過
- 文件一致性：已更新 DESIGN 與 godoc，明確限定淺層複製
- 剩餘風險：公開 `FromContext` 每次讀取增加一次 allocation；nested reference 不在保護範圍；遠端跨平台驗收待 commit／push

## 後續改善

- [ ] 另立 spec 清理 encoder 假契約與 SQL dead code
- [ ] 決定 README.en 維護或退場策略
