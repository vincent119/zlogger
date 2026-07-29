# 任務文件：建立檔案輸出安全邊界

Status: Complete

## Execution Context

- 意圖：阻止 FileName／filePrefix 路徑逸出與既有最終 symlink 跟隨，收緊新路徑權限，並提供敏感欄位 guardrail
- 本輪授權：使用者已指示 run task，依本文件執行 TDD、產品碼、文件與驗證
- 非目標：不處理原子 symlink race、Go／CI 升版、自訂 mode、自動 redaction、SQL、encoder、Context 或效能
- 已定決策：trusted base + untrusted leaf；共用 helper；既有 symlink 拒絕；新目錄 0700、新檔 0600；既有 mode 不變；新增 ErrUnsafeLogPath 與 Redacted
- 邊界：只允許後續 tasks 指定的安全 helper、Config、一般／分級輸出、fields、測試及兩份公開文件
- 關鍵檔案：`file_security.go`、`config.go`、`core.go`、`split_output.go`、`fields.go`
- 完成條件：requirements 的八個情境全數覆蓋；race、20 次安全測試、vet、格式、lint 通過；公開契約與剩餘風險文件一致

### Protected Behavior

- `New`、`Configure`、`NewSplitOutput`、`GetSplitCore` 與既有 Field API 簽章不變。
- 空 FileName／prefix 與安全 leaf name 保持可用。
- 日期檔名、分級檔名、level 路由、換檔、Sync、Close 與 worker 生命週期不變。
- 既有檔案仍 append，且 mode 不被改寫。
- 不新增依賴、不修改 `go.mod`、`go.sum`、`.github/` 或 Makefile。

### 邊界

#### Allowed Changes

實作階段限於：

- `file_security.go`
- `file_security_test.go`
- `config.go`
- `config_test.go`
- `core.go`
- `core_test.go`
- `split_output.go`
- `split_output_test.go`
- `fields.go`
- `fields_test.go`
- `README.md`
- `DESIGN.md`
- 本 spec 目錄內文件

#### Forbidden

- `encoder.go`、`context.go`、`zlogger.go` 及其測試
- `go.mod`、`go.sum`、`.github/`、Makefile
- `_workspace/` 既有審查報告
- 既有公開函式簽章與檔名格式
- os.Root、外部 dependency、自製 syscall、mode options
- 自動 redaction、SQL core、效能或 buffer 重構

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 建立 leaf 與 Config 驗收測試 | 無 | Complete | TDD Red／Green 已記錄 |
| T2 實作共享驗證與 error chain | T1 | Complete | race 與 20 次測試通過 |
| T3 建立一般／分級 symlink 與 mode 測試 | 無 | Complete | POSIX mode 與 symlink 已覆蓋 |
| T4 實作安全開檔與一般 file 整合 | T2、T3 | Complete | cleanup 與 append 保持 |
| T5 實作 SplitOutput 安全整合 | T2、T3、T4 | Complete | 換檔生命週期保持 |
| T6 新增 Redacted 與測試 | 無 | Complete | 不接收 value |
| T7 更新 README、DESIGN 與 godoc | T4、T5、T6 | Complete | 已揭露 TOCTOU |
| T8 完整驗證與邊界檢查 | T1 至 T7 | Complete | 未執行發布 |

## 實作任務

- [x] T1 建立 leaf-name 與 Config 驗收測試
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_security_test.go`、`config_test.go`
    - Forbidden：產品碼、其他測試與文件
  - Depends: 無
  - Context: 建立 table-driven tests，覆蓋 `../`、絕對路徑、正反斜線、`.`、`..`、NUL、Windows drive prefix、Unicode 安全名稱、空 FileName 與空 prefix。Config file output 錯誤需同時符合 ErrInvalidConfig 與 ErrUnsafeLogPath；console-only Config 不因未使用 FileName 失敗。
  - Verify:
    - `go test -count=1 -run 'Test(ValidateLogLeaf|ConfigValidateRejectsUnsafeFileName)' ./...`
    - Red 階段記錄失敗；Green 後 table 全數通過

- [x] T2 實作共享 leaf 驗證、containment 與錯誤分類
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_security.go`、`file_security_test.go`、`config.go`、`config_test.go`
    - Forbidden：開檔、mode、core、SplitOutput、fields 與公開文件
  - Depends: T1
  - Context: 新增 ErrUnsafeLogPath、validateLogLeaf、secureLogPath 與 0700／0600 private constants。Config.Validate 只在 file output 啟用時驗證 FileName，以多重 `%w` 保留兩種錯誤分類。所有平台都拒絕 `/` 與 `\\`。
  - Verify:
    - `go test -race -count=1 -run 'Test(ValidateLogLeaf|SecureLogPath|ConfigValidateRejectsUnsafeFileName)' ./...`
    - `go test -count=20 -run 'Test(ValidateLogLeaf|SecureLogPath)' ./...`

- [x] T3 建立一般／分級 containment、symlink 與 mode 驗收測試
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_security_test.go`、`core_test.go`、`split_output_test.go`
    - Forbidden：產品碼、其他測試與文件
  - Depends: 無
  - Context: 使用 t.TempDir 建立 base 與 outside。分別測試一般日期／自訂檔名、三個 split 檔、info 或 warn 目標 symlink、外部內容不變、新 mode group／other bits 為 0、既有 0640 mode 保持。symlink 或 mode 不適用平台明確 skip。
  - Verify:
    - `go test -count=1 -run 'Test(FileOutputsStayWithinBaseDirectory|FileOutputsRejectExistingSymlink|FileOutputsUsePrivatePermissions|FileOutputPreservesExistingPermissions|NewSplitOutputRejectsUnsafePrefix)' ./...`
    - Red 階段至少 traversal、symlink 或 mode 情境失敗

- [x] T4 實作安全開檔 helper 與一般 file output 整合
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_security.go`、`file_security_test.go`、`core.go`、`core_test.go`
    - Forbidden：SplitOutput、fields、Config 公開型別、go.mod 與其他產品檔
  - Depends: T2、T3
  - Context: openSecureLogFile 先 secureLogPath，再 Lstat 拒絕最終 symlink，最後以 0600 OpenFile。newFileCore 改用 helper；MkdirAll 使用 0700。不得 Chmod 既有資源，所有 error 以 `%w` 保留。
  - Verify:
    - `go test -race -count=1 -run 'TestFile(Output|Outputs)' ./...`
    - `go test -count=20 -run 'TestFileOutputsRejectExistingSymlink|TestFileOutputsUsePrivatePermissions' ./...`

- [x] T5 實作 SplitOutput prefix、mode 與安全開檔整合
  - Status: Complete
  - Boundary:
    - Allowed Changes：`split_output.go`、`split_output_test.go`、`file_security.go`
    - Forbidden：level routing、timer／worker 結構、公開簽章、其他產品檔
  - Depends: T2、T3、T4
  - Context: private constructor 在 MkdirAll 前驗證 prefix；MkdirAll 使用 0700。openSplitFiles 先產生既有 leaf 名稱，再使用 openSecureLogFile；中途 symlink 錯誤沿用 errors.Join 關閉已開檔案。不得改寫輪替與 Close 鎖順序。
  - Verify:
    - `go test -race -count=1 -run 'Test(NewSplitOutputRejectsUnsafePrefix|FileOutputsRejectExistingSymlink|SplitOutputCloseStopsRotation|GetSplitCoreRoutesLevels)' ./...`
    - `go test -count=20 -run 'Test(NewSplitOutputRejectsUnsafePrefix|SplitOutputCloseStopsRotation|GetSplitCoreRoutesLevels)' ./...`

- [x] T6 新增 Redacted helper 與驗收測試
  - Status: Complete
  - Boundary:
    - Allowed Changes：`fields.go`、`fields_test.go`
    - Forbidden：其他 Field helper、core 自動遮罩、文件與外部依賴
  - Depends: 無
  - Context: 新增 `Redacted(key string) Field`，只回傳固定 `[REDACTED]` zap string field。API 不接收 value，不依 key 猜測敏感性，不修改既有 String／Any／Reflect。
  - Verify:
    - `go test -race -count=1 -run 'TestRedactedField' ./...`
    - 測試 JSON encoding 後只出現 marker，不含任何測試秘密值

- [x] T7 更新 README、DESIGN 與 godoc
  - Status: Complete
  - Boundary:
    - Allowed Changes：`README.md`、`DESIGN.md`、`file_security.go`、`config.go`、`split_output.go`、`fields.go`
    - Forbidden：產品行為擴張、README.en.md、其他文件
  - Depends: T4、T5、T6
  - Context: 記錄 trusted base／untrusted leaf、拒絕字元、安全 mode、既有 mode 不變、ErrUnsafeLogPath、Redacted、禁止記錄 token／password／Authorization／cookie／完整個資，以及 Go 1.21 Lstat 的 TOCTOU 剩餘風險。不得宣稱自動 redaction 或完全防止 symlink race。
  - Verify:
    - `rg -n 'ErrUnsafeLogPath|Redacted|0700|0600|symlink|TOCTOU|token|密碼|Authorization|cookie|個資' README.md DESIGN.md file_security.go config.go split_output.go fields.go`
    - 文件逐項對照 requirements 的 threat model 與非目標

## 驗證任務

- [x] T8 驗收情境覆蓋
  - Verify: requirements.md 八個情境均有對應 selector；traversal 同時涵蓋 `/`、`\\`、絕對與 drive prefix

- [x] T9 回歸與穩定性驗證
  - Verify:
    - `go test -race -count=1 ./...`
    - `go test -count=20 -run 'Test(ValidateLogLeaf|SecureLogPath|ConfigValidateRejectsUnsafeFileName|FileOutputs|FileOutputPreserves|NewSplitOutputRejectsUnsafePrefix|RedactedField|GetSplitCoreRoutesLevels)' ./...`

- [x] T10 品質、安全與邊界檢查
  - `gofmt -d *.go` 無差異
  - `go vet ./...` 通過
  - `golangci-lint run ./...` 通過
  - `git diff --check` 通過
  - `git diff --stat` 只包含 Allowed Changes
  - `_workspace/` 未加入差異或 commit
  - 不安全輸入均在 I/O／goroutine 前失敗
  - symlink 外部內容不變，partial split files 已清理
  - 新資源 group／other mode bits 為 0，既有 mode 不變
  - 所有 error 支援 errors.Is，沒有新增 library panic
  - README、DESIGN、godoc 未過度承諾 symlink 或 redaction 能力

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 Execution Context
2. 目前未完成 task
3. Protected Behavior
4. Implementation Notes

不得掃描整個 `.specs` 目錄。定位命令：

```bash
rg -n '^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:' .specs/2026-07-29-11-18_BugFix-file-output-security-boundary
```

## Implementation Notes

- 2026-07-29：完成 requirements、design、tasks 初稿；尚未修改產品碼、測試、README 或 DESIGN。
- 2026-07-29：目前分支為 `security/file-output-boundary`；`main` 已同步至合併提交 `8d2bfe0`。
- 2026-07-29：確認 go.mod 與 CI 為 Go 1.21，雖然本機為 Go 1.26.5，本 spec 不使用 Go 1.24 的 os.Root。
- 2026-07-29：原子 symlink containment 另立工具鏈升版相依 spec；本批只拒絕建構時已存在的最終 symlink，並明確揭露 TOCTOU。
- 2026-07-29：既有未追蹤 `_workspace/` 不屬於本 spec，禁止加入提交。
- 2026-07-29：TDD Red 因 ErrUnsafeLogPath、leaf helper 與 Redacted 尚不存在而編譯失敗；完成共享安全元件與整合後轉為 Green。
- 2026-07-29：一般與分級輸出均拒絕 traversal 與既有最終 symlink；測試確認外部內容不變，空 prefix 與安全檔名保持可用。
- 2026-07-29：新目錄／檔案的 group 與 other bits 為 0，既有 0640 檔案 mode 保持且內容可追加。
- 2026-07-29：完整 race、20 次安全目標、go vet、gofmt 與 golangci-lint 均通過，覆蓋率 92.9%。
- 2026-07-29：差異限於 Allowed Changes；未修改 go.mod、go.sum、CI 或 `_workspace/`，未執行 commit、push 或發布。
- 2026-07-29：後續 `os.Root` containment 已由 PR #8 完成並合併為 `4d05e06`；GitHub Actions run `30427993668` 七項檢查全部通過。
- 2026-07-29：Context fields defensive copy 已由 PR #10 完成並合併為 `e1306af`；GitHub Actions run `30430536442` 七項檢查全部通過。
- 2026-07-29：encoder 契約與 SQL dead code 清理已由 PR #12 完成並合併為 `f7ae8f6`；GitHub Actions run `30432312003` 七項檢查全部通過。

## 驗證結果摘要

- 路徑驗證：通過；Unix／Windows traversal、NUL、`.`、`..`、絕對與 drive prefix 均有測試
- symlink：通過；一般與分級輸出拒絕既有最終 symlink，外部檔內容不變
- 權限：通過；新路徑採 private mode，既有 mode 不改寫
- 敏感欄位：通過；Redacted 固定輸出 `[REDACTED]` 且不接收 value
- 回歸驗證：通過；完整 race 與安全目標連續 20 次通過
- 品質檢查：通過；go vet、gofmt、golangci-lint 為 0 issues
- 測試覆蓋率：92.9%

## 後續改善

- [x] 升級 go.mod／CI 至 Go 1.25+，改用 os.Root 提供原子 containment
- [ ] 釘選 GitHub Actions 與 golangci-lint 版本，加入最小 permissions
- [ ] 評估可配置 DirPerm／FilePerm functional options
- [x] 修正 Context fields defensive copy
- [x] 清理 encoder 假契約與 SQL dead code
