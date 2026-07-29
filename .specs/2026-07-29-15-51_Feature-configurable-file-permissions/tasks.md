# 任務文件：可配置檔案輸出權限

Status: Complete

## Execution Context

- 意圖：以相容的 functional options 讓一般與分級輸出可設定新目錄／新檔案 mode，預設與安全邊界不變
- 本輪授權：使用者已指示 `task run`，依本文件執行 TDD、產品碼、文件與本機驗證；commit、push 與遠端驗收需另行授權
- 非目標：不改 Config schema、既有函式簽章、既有 mode、ownership／ACL、path containment、rotation lifecycle、CI 或 dependency
- 已定決策：新增 `*WithOptions` 入口；共用 `FileOutputOption`；預設 `0700`／`0600`；拒絕 other-write；options 在 I/O 前解析
- 邊界：後續實作只允許 option、file security、一般／分級輸出、直接測試、README、DESIGN 與本 spec
- 關鍵檔案：`file_options.go`、`file_security.go`、`core.go`、`split_output.go` 及其測試
- 完成條件：五個驗收情境、舊 API 簽章斷言、兩版 race、目標測試 20 次、make verify、遠端七項 CI 與文件一致性全部通過

### Protected Behavior

- `New`、`Configure`、`NewSplitOutput`、`GetSplitCore` 精確函式簽章與直接呼叫行為不變。
- `Config`、`ConfigPatch` 欄位、tags、DefaultConfig、Resolve、Merge 與 Validate 不變。
- 預設新目錄 `0700`、新檔案 `0600`，既有物件 mode 不變。
- leaf validation、`ErrUnsafeLogPath`、單一 `os.Root` batch containment 與 partial cleanup 不變。
- 一般／分級 append、檔名、level routing、rotation、Sync、Close 與 cleanup 不變。
- 不新增 dependency，不修改 go.mod、go.sum、CI、Makefile、lint 或 coverage gate。

### 邊界

#### Allowed Changes

- `file_options.go`
- `file_options_test.go`
- `file_security.go`
- `file_security_test.go`
- `core.go`
- `core_test.go`
- `split_output.go`
- `split_output_test.go`
- `README.md`
- `DESIGN.md`
- `.specs/2026-07-29-15-51_Feature-configurable-file-permissions/`
- T8 僅可勾選 `.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/tasks.md` 的 CI 釘選與 permission options 項目
- T8 僅可回填 `.specs/2026-07-29-11-48_Chore-go-toolchain-ci-baseline/tasks.md` 已有合併證據的 os.Root、Context 與 encoder／SQL 後續項目

#### Forbidden

- 修改既有公開函式簽章或 Config／ConfigPatch schema
- 對既有目錄或檔案執行 Chmod、Chown 或 ACL 操作
- 修改 `context.go`、`encoder.go`、`fields.go`、`zlogger.go` 或無關測試
- 修改 `go.mod`、`go.sum`、`.github/`、`Makefile`、`.golangci.yml` 或 Codecov
- 放寬 leaf 規則、移除 `os.Root`、改檔名／路由／rotation／lifecycle
- 接受 other-write、特殊 mode bits、nil option，或吞沒 validation／cleanup error
- 使用 sleep、無上限 retry、全域可變 test hook 或 Windows 整組 skip

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 建立 option 與相容性 TDD Red | 無 | Complete | 新 API 缺失造成預期編譯 Red |
| T2 實作 option resolver 與 rooted opener mode | T1 | Complete | 共用 validation 與 file mode 傳遞完成 |
| T3 整合一般輸出與 Configure | T2 | Complete | 舊入口、rollback 與全域發布不變 |
| T4 整合 SplitOutput 與 rotation | T2 | Complete | 初始與 rotation 共用 settings |
| T5 更新公開文件 | T3、T4 | Complete | umask、安全責任與 Windows 限制已記錄 |
| T6 本機完整驗證 | T1 至 T5 | Complete | 兩版 race、20 次與 verify 通過 |
| T7 遠端跨平台驗收 | T6 | Complete | PR #14 的七項 CI 全部通過 |
| T8 回填 spec 與歷史待辦 | T7 | Complete | 只更新明列 checkbox，並附合併證據 |

## 實作任務

- [x] T1 建立 option validation、API 相容與 mode 行為 Red tests
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_options_test.go`、`file_security_test.go`、`core_test.go`、`split_output_test.go`
    - Forbidden：產品碼、README、DESIGN、既有測試語意
  - Depends：無
  - Context：以編譯斷言固定四個舊入口的精確型別；建立 defaults、last-option-wins、nil option、特殊位元、缺 owner bits、other-write table。一般與分級 mode 測試需覆蓋新建、既有物件及 rotation；禁止用 sleep 驅動換檔。
  - Verify：
    - Red 證據顯示 `FileOutputOption`、`*WithOptions` 或 mode 傳遞尚不存在
    - `go test -count=1 -run 'Test(FileOutputOptions|NewWithOptions|SplitOutputWithOptions)' ./...`

- [x] T2 實作共用 option resolver 與 rooted opener mode
  - Status: Complete
  - Boundary:
    - Allowed Changes：`file_options.go`、`file_options_test.go`、`file_security.go`、`file_security_test.go`
    - Forbidden：core、SplitOutput、文件與其他產品碼
  - Depends：T1
  - Context：新增封閉式 `FileOutputOption`、`ErrInvalidFilePermission`、defaults 與 resolver。只允許 permission bits；dir 要求 owner `0700`、file 要求 owner `0600`；兩者拒絕 `0002`。rooted opener 接收已驗證 file mode，保留單一 root 與 partial cleanup。
  - Verify：
    - `go test -race -count=1 -run 'Test(FileOutputOptions|OpenRootedLogFiles)' ./...`
    - `go test -count=20 -run 'Test(FileOutputOptions|OpenRootedLogFiles)' ./...`

- [x] T3 整合一般輸出與 Configure options
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core.go`、`core_test.go`、必要的 `file_security_test.go`
    - Forbidden：Config schema、legacy Init 行為、SplitOutput、文件
  - Depends：T2
  - Context：新增 `NewWithOptions` 與 `ConfigureWithOptions`；舊入口以空 options 委派。options 必須在 Config validation 後、任何 MkdirAll 前完成；rollback、global publish、retry 與 cleanup 不變。console-only 仍驗證顯式 options，避免靜默忽略錯誤。
  - Verify：
    - `go test -race -count=1 -run 'Test(NewWithOptions|ConfigureWithOptions|LegacyInit|FileOutput)' ./...`
    - `go test -count=20 -run 'Test(NewWithOptions|ConfigureWithOptions)' ./...`

- [x] T4 整合 SplitOutput、GetSplitCore 與 rotation
  - Status: Complete
  - Boundary:
    - Allowed Changes：`split_output.go`、`split_output_test.go`、必要的 `file_security.go`／test
    - Forbidden：timer 模型、worker、mutex、檔名、level routing、Close／Sync 契約
  - Depends：T2
  - Context：新增兩個分級 `*WithOptions` 入口；舊入口使用 defaults。解析後 settings 保存於 SplitOutput 或 opener closure，初始三檔與每次 rotation 使用相同 file mode。測試沿用可控制 clock，不新增 sleep。
  - Verify：
    - `go test -race -count=1 -run 'Test(SplitOutputWithOptions|GetSplitCoreWithOptions|SplitOutputRotation|SplitOutputClose)' ./...`
    - `go test -count=20 -run 'Test(SplitOutputWithOptions|SplitOutputRotation)' ./...`

- [x] T5 更新 README、DESIGN 與 exported godoc
  - Status: Complete
  - Boundary:
    - Allowed Changes：`README.md`、`DESIGN.md`、新公開 API 所在檔案的 godoc、本 spec
    - Forbidden：README coverage badge、CI、dependency 與未實作能力
  - Depends：T3、T4
  - Context：提供 `0750`／`0640` 範例；說明 defaults、last-option-wins、拒絕規則、umask、既有 mode、Windows 限制與放寬權限責任。不得宣稱精確繞過 umask、修改既有權限或管理 ACL。
  - Verify：
    - `rg -n 'WithDirPerm|WithFilePerm|0700|0600|umask|Windows|既有' README.md DESIGN.md file_options.go core.go split_output.go`
    - godoc 與 requirements 的公開契約逐項一致

- [x] T6 本機完整驗證與邊界檢查
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：新增產品、測試或 workflow 變更
  - Depends：T1 至 T5
  - Context：執行最低與現行 Go race、目標測試 20 次、make verify、coverage 與 diff boundary；確認無 Chmod／Chown／ACL、新 dependency、Config schema 或既有簽章變更。
  - Verify：
    - Go 1.25.11：`go test -race -count=1 ./...`
    - Go 1.26.5：`go test -race -count=1 ./...`
    - `go test -count=20 -run 'Test(FileOutputOptions|NewWithOptions|ConfigureWithOptions|SplitOutputWithOptions|FileOutputOptionsPreserveExistingPermissions)' ./...`
    - `make verify`
    - `git diff --check`

- [x] T7 遠端 macOS／Windows 驗收
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過 CI 修改 workflow、skip、mode 契約或 sleep
  - Depends：T6
  - Context：經使用者授權 commit／push 後確認七項 CI 全部通過；Windows 必須通過 option validation、建立、rotation 與 cleanup，只允許 POSIX mode assertion skip。
  - Verify：`gh pr checks` 或 `gh run view` 顯示七個 jobs pass

- [x] T8 回填完成狀態與歷史待辦
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 spec；Execution Context 明列的兩份歷史 tasks checkbox
    - Forbidden：其他 spec、產品碼、測試或文件內容重寫
  - Depends：T7
  - Context：遠端 green 後將本 requirements／tasks 標記 Complete，附 PR、merge commit、run／job 證據。前置安全 spec 勾選已由 CI baseline 完成的 Action 釘選與本 spec 完成的 permission options；CI baseline spec 只回填已有合併證據的 os.Root、Context、encoder／SQL 項目。
  - Verify：文件狀態、checkbox 與 GitHub 證據一致

## 驗證任務

- [x] V1 公開契約與 validation
  - 四個舊入口精確函式型別不變
  - 新入口、options 與 sentinel godoc 完整
  - invalid modes 在 I/O 前以 `ErrInvalidFilePermission` 回傳

- [x] V2 filesystem mode 行為
  - defaults `0700`／`0600`
  - 自訂 `0750`／`0640`
  - umask 只會限縮 mode
  - 既有物件 mode 不變且 append 保留

- [x] V3 SplitOutput lifecycle
  - 初始三檔與 rotation 沿用相同 settings
  - 路由、transaction、Close、Sync 與 os.ErrClosed 不變
  - Windows cleanup 無 handle regression

- [x] V4 安全與邊界
  - leaf validation、os.Root containment、partial cleanup 不變
  - 無 Chmod、Chown、ACL、Config schema、新 dependency 或 CI 變更
  - README／DESIGN 不過度承諾

- [x] V5 品質與跨平台
  - Go 1.25.11／1.26.5 race
  - targeted tests 連續 20 次
  - fmt、vet、lint、coverage、benchmark
  - macOS 15、Windows 2025 遠端 CI

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 `Execution Context`
2. 目前未完成 task
3. `Protected Behavior`
4. `Implementation Notes`

不得預設掃描整個 `.specs` 目錄。定位命令：

```bash
rg -n '^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:' .specs/2026-07-29-15-51_Feature-configurable-file-permissions
```

## Implementation Notes

- 2026-07-29：從同步至 PR #13 merge commit `89680f2` 的 main 建立分支 `feature/configurable-file-permissions`。
- 2026-07-29：現況確認一般與分級輸出共用 `defaultLogDirMode=0700`、`defaultLogFileMode=0600`；新檔透過 `os.Root.OpenFile` 建立，既有 mode 不變。
- 2026-07-29：CI workflow 已以完整 Action SHA、golangci-lint v2.12.2 與 `permissions: contents: read` 完成供應鏈基線；前置安全 spec 的 CI 項目只是歷史 checkbox 未回填，不需重複實作。
- 2026-07-29：選擇新增 `*WithOptions` 入口，避免改動既有函式精確型別；不把 `os.FileMode` 加入 Config schema。
- 2026-07-29：requirements、design、tasks 已建立；依既有工作流程停在設計階段，尚未修改產品碼、測試、README 或 DESIGN。
- 2026-07-29：spec 標題、Status、Boundary、Depends 與 Implementation Notes 結構檢查通過；`git diff --check` 通過，分支差異只有本 spec 三份文件。
- 2026-07-29：使用者指示 `task run`，requirements 與 tasks 狀態改為 InProgress，從 T1 TDD Red 開始執行。
- 2026-07-29：T1 新增舊 API 精確簽章斷言、invalid mode table、四個新入口的 I/O 前拒絕，以及一般／分級／rotation／既有 mode 驗收；產品碼未修改時，目標測試因 `FileOutputOption`、resolver 與 `*WithOptions` 缺失產生預期編譯 Red。
- 2026-07-29：T2 新增封閉式 `FileOutputOption`、`WithDirPerm`、`WithFilePerm`、`ErrInvalidFilePermission` 與單一 resolver；逐一拒絕 nil、非 permission bits、缺 owner 必要權限及 other-write。rooted opener 改接收已驗證 file mode，既有 test seam 保持 `0600` wrapper。
- 2026-07-29：T3 新增 `NewWithOptions`、`ConfigureWithOptions` 與 settings-aware file core；舊 `New`、`Configure`、legacy Init、rollback、retry、global publish 與 cleanup 入口維持原簽章及行為。
- 2026-07-29：T4 新增 `NewSplitOutputWithOptions`、`GetSplitCoreWithOptions`；SplitOutput 保存解析後 settings，初始三檔與可控制 clock 觸發的下一日 rotation 均沿用自訂 file mode，既有 opener 測試介面以 wrapper 相容。
- 2026-07-29：T2 至 T4 的 permission、一般輸出、Configure、分級公開入口、rooted opener、rotation、Close 與既有 mode 目標 race 通過；相同核心 selectors 連續 20 次通過。
- 2026-07-29：T5 更新 README、DESIGN 與 exported godoc，記錄 `0750`／`0640` 範例、預設值、last-option-wins、拒絕規則、umask、既有 mode、Windows 限制及敏感日誌責任。
- 2026-07-29：T6 的 Go 1.26.5 完整 race 通過；Go 1.25.11 初次因環境 `GOSUMDB=off` 無法驗證官方 toolchain，僅對該命令使用 `GOSUMDB=sum.golang.org` 後完整 race 通過，未修改持久環境設定。
- 2026-07-29：第一次 `make verify` 在 lint 階段揭露五個刻意建立 `0640` 參考檔的 G306 finding，以及一個已無 caller 的 private default opener；逐處加入受控測試理由並移除 unused wrapper，未降低 lint 規則。
- 2026-07-29：最終 `make verify GOLANGCI_LINT=/private/tmp/zlogger-tools/golangci-lint` 通過 fmt-check、vet、golangci-lint v2.12.2、race、92.5% coverage gate 與 benchmark smoke；`make clean` 已移除 coverage 產物。
- 2026-07-29：`go doc` 確認七個新增 exported identifiers 的 godoc 可見；`git diff --check` 通過，差異只含 Allowed Changes，未修改 Config schema、dependency、CI、Makefile、lint、Codecov 或無關產品檔，產品碼無 Chmod／Chown／umask 操作。
- 2026-07-29：PR #14 已合併為 `d1a5618`；GitHub Actions run `30434997130` 的 macOS 15、Windows 2025、Go 1.25.11／1.26.5 race、靜態與格式、coverage／Codecov、benchmark 共七項工作全部通過，完成 T7、V3 與 V5。
- 2026-07-29：完成 T8；本 spec 狀態已回填為 Complete，並依明列邊界更新檔案輸出安全規格的 CI 釘選與 permission options，以及 CI 基線規格中已有合併證據的 os.Root、Context、encoder／SQL 後續項目。

## 驗證結果摘要

- 新行為驗證：通過；目標 race 與相同 selectors 連續 20 次通過
- 回歸驗證：通過；Go 1.25.11／1.26.5 完整 race 與 `make verify` 通過
- 文件一致性：已確認 README、DESIGN、godoc、requirements、design 與 tasks
- 剩餘風險：無阻擋風險；僅保留下列不屬於本 spec 的後續改善

## 後續改善

- [ ] 評估是否在下一個 major version 將 options 整合至單一建構入口
- [ ] 評估 Unix group ownership 與 Kubernetes `fsGroup` 的部署文件，不納入 library 自動管理
