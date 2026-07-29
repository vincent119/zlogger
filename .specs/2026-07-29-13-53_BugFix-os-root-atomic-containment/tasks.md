# 任務文件：以 os.Root 建立原子檔案 containment

Status: Complete

## Execution Context

- 意圖：以標準庫 `os.Root` 消除日誌 leaf 在檢查與開檔間被替換後逸出 trusted base 的風險
- 本輪授權：使用者已指示 `go`，依本文件執行 TDD、產品碼、文件與本機驗證；commit、push 與遠端驗收需另行授權
- 非目標：不改公開 API、base trust、leaf 規則、mode、SplitOutput lifecycle、CI、coverage badge、Context、encoder 或 SQL
- 已定決策：每批單一 root；一般輸出一檔一批；SplitOutput 三檔一批；穩定 symlink 維持拒絕；root 不成為長期 runtime owner
- 邊界：後續實作限於檔案安全 helper、一般／分級輸出整合、對應測試、README、DESIGN 與本 spec
- 關鍵檔案：`file_security.go`、`file_security_test.go`、`core.go`、`split_output.go`
- 完成條件：八個驗收情境、兩版 race、20 次安全與 rotation tests、make verify、macOS 15／Windows 2025 遠端 CI 全部通過

### Protected Behavior

- `New`、`Configure`、`NewSplitOutput`、`GetSplitCore` 與所有 exported API 簽章不變。
- trusted base／untrusted leaf 契約與 leaf validation 規則不變。
- 空 FileName／prefix、安全 Unicode leaf、日期與分級檔名格式不變。
- 新目錄 `0700`、新檔 `0600`、既有 mode 不變、append 行為不變。
- DEBUG／INFO／WARN／ERROR 路由、rotation transaction、worker、Sync 與 Close 不變。
- `ErrUnsafeLogPath` 與 Config `ErrInvalidConfig` error chain 不變。
- 不新增 dependency，不修改 go.mod、go.sum、CI、Makefile 或 coverage gate。

### 邊界

#### Allowed Changes

- `file_security.go`
- `file_security_test.go`
- `core.go`
- `core_test.go`
- `split_output.go`
- `split_output_test.go`
- `README.md`
- `DESIGN.md`
- `.specs/2026-07-29-13-53_BugFix-os-root-atomic-containment/`

#### Forbidden

- exported API、Config schema、錯誤 sentinel 或檔名格式變更
- `context.go`、`encoder.go`、`fields.go`、`zlogger.go` 與無關測試
- `go.mod`、`go.sum`、`.github/`、Makefile、`.golangci.yml`
- Codecov、coverage badge、workflow permission 或新 secret
- 長期保存 `*os.Root`、外部 dependency、自製 syscall、平台專用 no-follow
- mode options、自動 redaction、SQL、Context 或效能重構
- sleep、無上限 retry、Windows 整組 skip 或忽略 cleanup error

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 建立 os.Root TDD Red | 無 | Complete | 缺少 rooted batch API，編譯 Red 已保存 |
| T2 實作批次 rooted opener | T1 | Complete | race 與連續 20 次通過 |
| T3 整合一般 file output | T2 | Complete | race 與連續 20 次通過 |
| T4 整合 SplitOutput | T2 | Complete | 三檔共用 root，rotation 測試通過 |
| T5 更新安全文件 | T3、T4 | Complete | 契約與限制已同步 |
| T6 本機完整驗證 | T1 至 T5 | Complete | 兩版 race、verify、20 次通過 |
| T7 遠端跨平台驗收 | T6 | Pending | 實際 push 後執行 |
| T8 回填 spec 完成狀態 | T7 | Pending | 附 run／job 證據 |

## 實作任務

- [x] T1 建立 os.Root containment 與 ownership Red tests
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_security_test.go`、必要的 package-private test seam
    - Forbidden：產品開檔行為、core、SplitOutput、文件
  - Depends：無
  - Context：建立穩定 symlink、檢查後並行替換、每批單一 root、第二／第三檔失敗與 root close failure tests。外部 sentinel 必須保持；不使用 sleep 猜測時序。若需 seam，只能控制安全 opener 的檢查／開檔點，不得形成公開 API 或全域可變狀態。
  - Verify：
    - Red 證據明確顯示現行完整路徑 `os.OpenFile` 缺少原子 containment 或批次 root 能力
    - `go test -count=1 -run 'Test(OpenRootedLogFiles|RootedFileOpen)' ./...`

- [x] T2 實作交易式批次 rooted opener
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_security.go`、`file_security_test.go`
    - Forbidden：core、SplitOutput、公開 API 與其他產品檔
  - Depends：T1
  - Context：新增 `openRootedLogFiles(baseDir string, leaves ...string)`。全部 leaf 先驗證；每批只呼叫一次 `os.OpenRoot`；以 `Root.Lstat` 維持穩定 symlink 拒絕，再以 `Root.OpenFile` 開檔。任何 open／root Close error 都關閉 partial files 並以 errors.Join 保留。舊 opener 在 T3、T4 caller 遷移完成後移除，避免中間 task 破壞編譯。
  - Verify：
    - `rg -n 'os\.OpenFile|os\.OpenRoot|Root\.OpenFile|openRootedLogFiles' file_security.go core.go split_output.go`
    - `go test -race -count=1 -run 'Test(OpenRootedLogFiles|RootedFileOpen|ValidateLogLeaf)' ./...`
    - `go test -count=20 -run 'Test(OpenRootedLogFiles|RootedFileOpen)' ./...`

- [x] T3 整合一般 file output
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core.go`、`core_test.go`、必要的 `file_security_test.go`
    - Forbidden：SplitOutput、公開簽章、encoder 與 Config 行為
  - Depends：T2
  - Context：`newFileCore` 維持 MkdirAll、日期 fallback、encoder 與 file ownership，只將一個 leaf 交給批次 rooted opener 並接收唯一 file。保留既有 append、mode、cleanup 與 Windows TempDir 行為。
  - Verify：
    - `go test -race -count=1 -run 'Test(FileOutput|FileOutputs|InitLogger_WithFile)' ./...`
    - `go test -count=20 -run 'TestRootedFileOutput|TestFileOutputPreservesExistingPermissions' ./...`

- [x] T4 整合 SplitOutput 三檔批次
  - Status: Complete
  - Boundary:
    - Allowed Changes：`split_output.go`、`split_output_test.go`、必要的 `file_security.go`／test
    - Forbidden：timer、worker、mutex、level routing、公開簽章與其他產品檔
  - Depends：T2
  - Context：`openSplitFiles` 一次傳入 info、warn、error 三個 leaf，依固定順序建立 `splitFileSet`。helper 負責 partial cleanup；保留 `splitFileOpener` 注入點、換檔交易、Close 冪等與 os.ErrClosed 行為。
  - Verify：
    - `go test -race -count=1 -run 'Test(RootedSplitOutput|SplitOutputCloseStopsRotation|GetSplitCoreRoutesLevels)' ./...`
    - `go test -count=20 -run 'Test(RootedSplitOutput|SplitOutputCloseStopsRotation|SplitOutputRotation)' ./...`

- [x] T5 更新 README 與 DESIGN 安全契約
  - Status: Complete
  - Boundary:
    - Allowed Changes：`README.md`、`DESIGN.md`、本 spec
    - Forbidden：README.en.md、coverage badge、CI 與產品碼
  - Depends：T3、T4
  - Context：移除「仍採 Lstat 後完整路徑 OpenFile」描述，改為 os.Root root-relative containment。明列 trusted base、root 內 symlink、mount boundary、特殊平台與 OpenRoot 前 base replacement 限制；不得使用 sandbox、完全防止或原子 no-follow 等過度承諾。
  - Verify：
    - `rg -n 'os.Root|TOCTOU|trusted|base|symlink|mount|Lstat|OpenFile' README.md DESIGN.md`
    - 文件逐項對照 requirements 威脅模型與實作

- [x] T6 本機完整驗證與邊界檢查
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：新增產品或測試變更
  - Depends：T1 至 T5
  - Context：執行兩版 race、targeted 20 次、make verify、coverage 與 diff boundary。確認無完整目標路徑 `os.OpenFile` 日誌入口、無 root／file 洩漏、無產物。
  - Verify：
    - Go 1.25.11：`go test -race -count=1 ./...`
    - Go 1.26.5：`go test -race -count=1 ./...`
    - `make verify`
    - `git diff --check`
    - coverage >= 90%

- [x] T7 遠端 macOS／Windows 驗收
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過 CI 修改 workflow、skip 或 sleep
  - Depends：T6
  - Context：經使用者授權 commit／push 後，確認既有七項 CI 全部通過，Windows TempDir cleanup 無 handle regression。
  - Verify：`gh pr checks` 或 `gh run view` 顯示所有 jobs pass

- [x] T8 回填完成狀態
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 spec；前置安全 spec 只更新 os.Root 後續項目
    - Forbidden：其他 spec 或產品碼
  - Depends：T7
  - Context：遠端 green 後將 requirements、tasks 與前置安全 spec 待辦標記完成，附 run／job 證據。
  - Verify：文件狀態與 GitHub 結果一致

## 驗證任務

- [x] V1 安全不變量
  - 穩定外部 symlink 維持 ErrUnsafeLogPath
  - 並行替換不修改 root 外 sentinel
  - Root.OpenFile 是共同開檔入口

- [x] V2 資源所有權
  - 每批只建立並關閉一個 root
  - partial files 全部關閉
  - root Close error 不被忽略
  - Windows TempDir cleanup 通過

- [x] V3 相容性
  - 安全 leaf、空 FileName／prefix 可用
  - append、mode、日期與分級檔名不變
  - rotation、level routing、Sync、Close 不變

- [x] V4 品質與跨平台
  - Go 1.25.11／1.26.5 race
  - targeted tests `-count=20`
  - fmt、vet、lint、coverage、benchmark
  - macOS 15、Windows 2025

- [x] V5 邊界
  - diff 只含 Allowed Changes
  - 無 dependency、CI、Makefile 或 coverage badge 變更
  - 無 sleep、retry、全域 test hook 或平台整組 skip
  - README、DESIGN 不過度承諾

## 實作中斷恢復

恢復時優先讀取：

1. 本文件 Execution Context
2. 目前未完成 task
3. Protected Behavior
4. Implementation Notes

定位命令：

```bash
rg -n '^#|^##|^###|Boundary|Depends|Status|Implementation Notes' .specs/2026-07-29-13-53_BugFix-os-root-atomic-containment
```

## Implementation Notes

- 2026-07-29：從最新 main 建立分支 `bugfix/os-root-atomic-containment`。
- 2026-07-29：確認 module 最低 Go 1.25.0；本機 Go 1.26.5 darwin/arm64，標準庫提供 `os.OpenRoot`、`Root.Lstat` 與 `Root.OpenFile`。
- 2026-07-29：現行 `openSecureLogFile` 以完整目標路徑執行 Lstat 後 OpenFile，仍存在 leaf replacement TOCTOU；一般與分級輸出共用此入口。
- 2026-07-29：決定每批單一 root，root 在檔案建立完成後立即關閉，不擴張 Instance 或 SplitOutput runtime lifecycle。
- 2026-07-29：README coverage badge 由提交 `8e1fdc0` 移除；coverage gate 與 `make coverage-check` 仍存在，本 spec 不恢復 badge。
- 2026-07-29：完成 requirements、design、tasks；依使用者要求停在設計階段，尚未修改產品碼、測試、README 或 DESIGN。
- 2026-07-29：Codecov 已由獨立 PR #7 恢復並合併；coverage job 成功上傳 report。此分支已 rebase 至 merge commit `445597d`，Codecov 不屬於本 spec 差異。
- 2026-07-29：使用者再次指示 `go`，spec 狀態改為 InProgress，從 T1 TDD Red 開始執行；commit、push 與遠端驗收仍需另行授權。
- 2026-07-29：T1 新增檢查後 symlink 替換、每批單一 root、partial file failure 與 root close failure 測試；Red 因 `rootedDirectory`、`openRootedLogFilesWith` 尚不存在而編譯失敗，未修改產品開檔行為。
- 2026-07-29：T2 新增每批單一 `os.Root` 的交易式 opener；穩定 symlink 維持 ErrUnsafeLogPath，檢查後替換不逸出 root，partial files 與 root close error 完整回收。目標 race 與連續 20 次測試通過。
- 2026-07-29：T3 將一般 file output 遷移至 rooted batch opener；穩定 symlink、append、mode、全域初始化與 cleanup selectors 的 race 及連續 20 次測試通過。
- 2026-07-29：T4 將 SplitOutput 每批 info、warn、error 三檔改為共用單一 root，並移除舊完整路徑 opener；路由、換檔、Close 的 race 與連續 20 次測試通過。
- 2026-07-29：T5 更新 README 與 DESIGN；文件說明穩定 symlink 拒絕、並行替換 containment，以及 base trust、root 內 symlink、mount boundary 與特殊平台限制，未宣稱完整 sandbox。
- 2026-07-29：T6 完成；Go 1.25.11 與 1.26.5 的完整 race 均通過，安全與 rotation 目標測試合併執行 20 次通過。
- 2026-07-29：`make verify` 通過 fmt-check、vet、golangci-lint v2.12.2、race、92.7% coverage 與 benchmark smoke；`git diff --check` 通過，無測試產物。
- 2026-07-29：分支差異只包含 Allowed Changes；產品碼已無完整目標路徑 `os.OpenFile` 或舊 opener，未修改 dependency、CI、Makefile、coverage badge 或公開 API。T7 保留等待 commit／push 後的遠端 macOS 15 與 Windows 2025 驗收。
- 2026-07-29：PR #8 已合併為 `4d05e06`；GitHub Actions run `30427993668` 的七項檢查全部通過，包含 Go 1.25.11／1.26.5 race、macOS 15、Windows 2025、靜態分析、coverage gate 與 benchmark smoke，完成 T7 與 V4。
- 2026-07-29：完成 T8；requirements 與 tasks 狀態改為 Complete，並回填前置安全 spec 的 `os.Root` 後續項目。產品碼與其他後續改善未修改。

## 後續改善

- [x] README coverage badge 已由獨立 PR #7 恢復
- [ ] 完成 Context fields defensive copy
- [ ] 清理 encoder 假契約與 SQL dead code
