# 任務文件：以 os.Root 建立原子檔案 containment

Status: Planned

## Execution Context

- 意圖：以標準庫 `os.Root` 消除日誌 leaf 在檢查與開檔間被替換後逸出 trusted base 的風險
- 本輪授權：只建立 requirements、design、tasks；不修改產品碼、測試、README 或 DESIGN
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
| T1 建立 os.Root TDD Red | 無 | Pending | 保存外部 sentinel 與 cleanup 證據 |
| T2 實作批次 rooted opener | T1 | Pending | 每批單一 root、交易式 cleanup |
| T3 整合一般 file output | T2 | Pending | 公開契約與 owner 不變 |
| T4 整合 SplitOutput | T2 | Pending | 三檔共用 root、rotation 不變 |
| T5 更新安全文件 | T3、T4 | Pending | 不過度承諾 os.Root |
| T6 本機完整驗證 | T1 至 T5 | Pending | 兩版 race、verify、20 次 |
| T7 遠端跨平台驗收 | T6 | Pending | 實際 push 後執行 |
| T8 回填 spec 完成狀態 | T7 | Pending | 附 run／job 證據 |

## 實作任務

- [ ] T1 建立 os.Root containment 與 ownership Red tests
  - Status: Pending
  - Boundary:
    - Allowed Changes：`file_security_test.go`、必要的 package-private test seam
    - Forbidden：產品開檔行為、core、SplitOutput、文件
  - Depends：無
  - Context：建立穩定 symlink、檢查後並行替換、每批單一 root、第二／第三檔失敗與 root close failure tests。外部 sentinel 必須保持；不使用 sleep 猜測時序。若需 seam，只能控制安全 opener 的檢查／開檔點，不得形成公開 API 或全域可變狀態。
  - Verify：
    - Red 證據明確顯示現行完整路徑 `os.OpenFile` 缺少原子 containment 或批次 root 能力
    - `go test -count=1 -run 'Test(OpenRootedLogFiles|RootedFileOpen)' ./...`

- [ ] T2 實作交易式批次 rooted opener
  - Status: Pending
  - Boundary:
    - Allowed Changes：`file_security.go`、`file_security_test.go`
    - Forbidden：core、SplitOutput、公開 API 與其他產品檔
  - Depends：T1
  - Context：新增 `openRootedLogFiles(baseDir string, leaves ...string)`。全部 leaf 先驗證；每批只呼叫一次 `os.OpenRoot`；以 `Root.Lstat` 維持穩定 symlink 拒絕，再以 `Root.OpenFile` 開檔。任何 open／root Close error 都關閉 partial files 並以 errors.Join 保留。移除不再具產品用途的完整路徑 opener／containment helper。
  - Verify：
    - `rg -n 'os\.OpenFile|os\.OpenRoot|Root\.OpenFile|openRootedLogFiles' file_security.go core.go split_output.go`
    - `go test -race -count=1 -run 'Test(OpenRootedLogFiles|RootedFileOpen|ValidateLogLeaf)' ./...`
    - `go test -count=20 -run 'Test(OpenRootedLogFiles|RootedFileOpen)' ./...`

- [ ] T3 整合一般 file output
  - Status: Pending
  - Boundary:
    - Allowed Changes：`core.go`、`core_test.go`、必要的 `file_security_test.go`
    - Forbidden：SplitOutput、公開簽章、encoder 與 Config 行為
  - Depends：T2
  - Context：`newFileCore` 維持 MkdirAll、日期 fallback、encoder 與 file ownership，只將一個 leaf 交給批次 rooted opener 並接收唯一 file。保留既有 append、mode、cleanup 與 Windows TempDir 行為。
  - Verify：
    - `go test -race -count=1 -run 'Test(FileOutput|FileOutputs|InitLogger_WithFile)' ./...`
    - `go test -count=20 -run 'TestRootedFileOutput|TestFileOutputsRejectExistingSymlink|TestFileOutputPreservesExistingPermissions' ./...`

- [ ] T4 整合 SplitOutput 三檔批次
  - Status: Pending
  - Boundary:
    - Allowed Changes：`split_output.go`、`split_output_test.go`、必要的 `file_security.go`／test
    - Forbidden：timer、worker、mutex、level routing、公開簽章與其他產品檔
  - Depends：T2
  - Context：`openSplitFiles` 一次傳入 info、warn、error 三個 leaf，依固定順序建立 `splitFileSet`。helper 負責 partial cleanup；保留 `splitFileOpener` 注入點、換檔交易、Close 冪等與 os.ErrClosed 行為。
  - Verify：
    - `go test -race -count=1 -run 'Test(RootedSplitOutput|SplitOutputCloseStopsRotation|GetSplitCoreRoutesLevels)' ./...`
    - `go test -count=20 -run 'Test(RootedSplitOutput|SplitOutputCloseStopsRotation|SplitOutputRotation)' ./...`

- [ ] T5 更新 README 與 DESIGN 安全契約
  - Status: Pending
  - Boundary:
    - Allowed Changes：`README.md`、`DESIGN.md`、本 spec
    - Forbidden：README.en.md、coverage badge、CI 與產品碼
  - Depends：T3、T4
  - Context：移除「仍採 Lstat 後完整路徑 OpenFile」描述，改為 os.Root root-relative containment。明列 trusted base、root 內 symlink、mount boundary、特殊平台與 OpenRoot 前 base replacement 限制；不得使用 sandbox、完全防止或原子 no-follow 等過度承諾。
  - Verify：
    - `rg -n 'os.Root|TOCTOU|trusted|base|symlink|mount|Lstat|OpenFile' README.md DESIGN.md`
    - 文件逐項對照 requirements 威脅模型與實作

- [ ] T6 本機完整驗證與邊界檢查
  - Status: Pending
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

- [ ] T7 遠端 macOS／Windows 驗收
  - Status: Pending
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過 CI 修改 workflow、skip 或 sleep
  - Depends：T6
  - Context：經使用者授權 commit／push 後，確認既有七項 CI 全部通過，Windows TempDir cleanup 無 handle regression。
  - Verify：`gh pr checks` 或 `gh run view` 顯示所有 jobs pass

- [ ] T8 回填完成狀態
  - Status: Pending
  - Boundary:
    - Allowed Changes：本 spec；前置安全 spec 只更新 os.Root 後續項目
    - Forbidden：其他 spec 或產品碼
  - Depends：T7
  - Context：遠端 green 後將 requirements、tasks 與前置安全 spec 待辦標記完成，附 run／job 證據。
  - Verify：文件狀態與 GitHub 結果一致

## 驗證任務

- [ ] V1 安全不變量
  - 穩定外部 symlink 維持 ErrUnsafeLogPath
  - 並行替換不修改 root 外 sentinel
  - Root.OpenFile 是共同開檔入口

- [ ] V2 資源所有權
  - 每批只建立並關閉一個 root
  - partial files 全部關閉
  - root Close error 不被忽略
  - Windows TempDir cleanup 通過

- [ ] V3 相容性
  - 安全 leaf、空 FileName／prefix 可用
  - append、mode、日期與分級檔名不變
  - rotation、level routing、Sync、Close 不變

- [ ] V4 品質與跨平台
  - Go 1.25.11／1.26.5 race
  - targeted tests `-count=20`
  - fmt、vet、lint、coverage、benchmark
  - macOS 15、Windows 2025

- [ ] V5 邊界
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

## 後續改善

- [ ] 另立 README coverage badge 策略文件變更
- [ ] 完成 Context fields defensive copy
- [ ] 清理 encoder 假契約與 SQL dead code
