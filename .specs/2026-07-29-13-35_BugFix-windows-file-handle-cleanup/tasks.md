# 任務文件：修正 Windows 測試檔案 handle 洩漏

Status: InProgress

## Execution Context

- 意圖：修正 Windows 2025 portability job 因測試 file handle 未關閉而失敗
- 本輪授權：使用者已指示 `go`，依本文件執行實作與本機驗證；commit、push 與遠端驗收需另行授權
- 非目標：不修改 runtime lifecycle、公開 API、CI matrix、Go 版本、os.Root 或 dependency
- 已定決策：不 skip、不 sleep、不 retry；TempDir 後註冊 cleanup；buildFileCore 回傳 owned file；cleanup error 必須回報
- 邊界：只修改 core.go 的 package-private test entry、core_test.go、兩份 spec tasks
- 關鍵檔案：`core.go`、`core_test.go`
- 完成條件：七個原失敗測試與 Windows 2025 job 通過，完整品質基線不回歸

### Protected Behavior

- `New`、`Configure`、`Init`、`Instance.Close` 與所有 exported API 簽章不變。
- runtime logger resource ownership 與 cleanup 順序不變。
- 日誌格式、level routing、SplitOutput、檔案權限、leaf validation 與 symlink 行為不變。
- buildFileCore 的 nil Config、空 LogPath fallback 與 panic-on-build-error package-private 行為保持。
- `.github/workflows/ci.yml`、`go.mod`、`go.sum`、Makefile、lint 與 coverage 設定不變。

### 邊界

#### Allowed Changes

- `core.go`
- `core_test.go`
- `context_test.go`、`zlogger_test.go`，只允許同步 global reset helper 的參數
- `.specs/2026-07-29-13-35_BugFix-windows-file-handle-cleanup/`
- `.specs/2026-07-29-11-48_Chore-go-toolchain-ci-baseline/tasks.md`，只在遠端 Windows job 通過後更新 T9 與完成狀態

#### Forbidden

- 任何 exported API 或 runtime lifecycle 行為變更
- `.github/workflows/ci.yml`、`go.mod`、`go.sum`、Makefile、`.golangci.yml`
- Windows skip、sleep、retry 或忽略 RemoveAll／Close error
- os.Root、path security、mode、symlink、encoder、SQL、Context 或效能重構
- 新增 dependency

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T1 保存 Windows Red 證據 | 無 | Complete | 既有 CI log 已確認 |
| T2 修正 global init 測試 cleanup | T1 | Complete | targeted 20 次通過 |
| T3 修正 buildFileCore ownership | T1 | Complete | 四個 owner 均註冊 cleanup |
| T4 本機回歸與穩定性 | T2、T3 | Complete | 兩版 race、20 次、verify 通過 |
| T5 遠端 Windows 驗收 | T4 | Pending | 必須實際 push |
| T6 回填兩份 spec 狀態 | T5 | Pending | Windows green 後執行 |

## 實作任務

- [x] T1 保存 Windows 失敗證據
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：產品碼與測試
  - Depends：無
  - Context：記錄 run 30425061820／job 90489619085 的七個失敗 selector、Go／runner 與共同 unlinkat handle-in-use error，作為 Red。
  - Verify：失敗全部發生在 TempDir RemoveAll；沒有產品 assertion failure

- [x] T2 修正 global init file-output 測試 cleanup
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core_test.go`；`context_test.go`、`zlogger_test.go` 只允許同步 reset helper 呼叫
    - Forbidden：`core.go`、其他測試與產品行為
  - Depends：T1
  - Context：讓 reset helper 可回報 global cleanup error；在三個 file-output init 測試的 TempDir 建立後註冊 t.Cleanup。cleanup 必須晚於 TempDir 註冊，使 LIFO 先關 logger。
  - Verify：`go test -count=20 -run 'TestInitLogger_With(FileOutput|FileAndConsoleOutput|AllOptions)$' ./...`；沒有忽略 cleanup error

- [x] T3 修正 buildFileCore owned file lifecycle
  - Status: Complete
  - Boundary:
    - Allowed Changes：`core.go`、`core_test.go`
    - Forbidden：newFileCore、runtime caller、公開 API 與其他產品檔
  - Depends：T1
  - Context：buildFileCore 同時回傳 core 與 `*os.File`；四個測試立即以共用 helper 註冊 Close。移除 close 前的手動 RemoveAll；保留既有 fallback 與 panic 行為。
  - Verify：`rg -n 'buildFileCore\(' core.go core_test.go` 所有呼叫均接收 file；`go test -count=20 -run 'TestBuildFileCore_(JSONFormat|ConsoleFormat|WithFileName|EmptyLogPath)$' ./...`

- [x] T4 本機完整驗證
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：產品與測試新增變更
  - Depends：T2、T3
  - Context：執行 targeted 20 次、Go 1.25.11／1.26.5 race、make verify、lint 與 diff boundary。
  - Verify：所有命令通過；coverage >= 90%；無 workflow、module 或 dependency diff

- [ ] T5 遠端 Windows 2025 驗收
  - Status: Pending
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過 CI 修改 workflow 或加平台 skip
  - Depends：T4
  - Context：經使用者授權 commit／push 後，確認 Windows 2025 portability job 實際執行七個測試並通過；macOS 與其他 jobs 也需 green。
  - Verify：`gh pr checks` 或 `gh run view` 顯示所有 jobs pass

- [ ] T6 回填 spec 完成狀態
  - Status: Pending
  - Boundary:
    - Allowed Changes：本 tasks、requirements；前置 CI tasks 的 T9 與狀態
    - Forbidden：其他 spec 內容與產品碼
  - Depends：T5
  - Context：只有遠端 Windows green 後，將本 spec 與前置 CI spec T9 標記 Complete，附 run／job 證據。
  - Verify：兩份 tasks 狀態與 GitHub 結果一致

## 驗證任務

- [x] V1 Targeted lifecycle
  - 三個 init file tests `-count=20`
  - 四個 buildFileCore tests `-count=20`
  - cleanup error 不被忽略

- [x] V2 完整回歸
  - Go 1.25.11：`go test -race -count=1 ./...`
  - Go 1.26.5：`go test -race -count=1 ./...`
  - `make verify`
  - coverage >= 90%

- [x] V3 邊界與安全
  - `git diff --check`
  - diff 只含 Allowed Changes
  - 無 `runtime.GOOS` skip、sleep、retry
  - workflow、module、dependency 與公開 API 不變
  - 每個新 file owner 都有對應 cleanup

- [ ] V4 遠端跨平台
  - Windows 2025 pass
  - macOS 15 pass
  - race、lint、coverage、benchmark jobs pass

## 實作中斷恢復

恢復時優先讀取：

1. 本文件 Execution Context
2. 目前未完成 task
3. Protected Behavior
4. Implementation Notes

定位命令：

```bash
rg -n '^#|^##|^###|Boundary|Depends|Status|Implementation Notes' .specs/2026-07-29-13-35_BugFix-windows-file-handle-cleanup
```

## Implementation Notes

- 2026-07-29：建立分支 `bugfix/windows-file-handle-cleanup`。
- 2026-07-29：完成 requirements、design、tasks 初稿；尚未修改 core.go、core_test.go 或前置 spec。
- 2026-07-29：PR #4 run 30425061820 只有 Windows 2025 job 90489619085 失敗；其他六類 jobs 通過。
- 2026-07-29：七個失敗均為 TempDir RemoveAll 嘗試刪除仍開啟日誌檔時回傳 handle-in-use，沒有產品 assertion failure。
- 2026-07-29：確認 buildFileCore 只由 core_test.go 四個測試使用，package-private 簽章調整不影響 exported API。
- 2026-07-29：使用者指示 `go`，spec 狀態改為 InProgress；T1 Red 證據確認七個 failure 全部發生於 Windows TempDir cleanup，沒有產品 assertion failure。
- 2026-07-29：T2 首次編譯顯示 resetGlobalState 為跨測試檔 helper；為了讓 cleanup error 可由 testing.TB 回報，將 context_test.go、zlogger_test.go 的機械呼叫調整納入 Allowed Changes，不擴張產品範圍。
- 2026-07-29：T2 讓 resetGlobalState 接收 testing.TB 並回報 cleanup error；三個 init file-output 測試在 TempDir 後註冊 global cleanup，targeted 測試連續 20 次通過。
- 2026-07-29：T3 讓 buildFileCore 回傳 owned *os.File；四個呼叫點都使用 registerFileCleanup，並移除 close 前忽略錯誤的 RemoveAll；targeted 測試連續 20 次通過。
- 2026-07-29：T4 完成；七個目標測試合併執行 20 次通過，Go 1.25.11 與 1.26.5 的 `go test -race -count=1 ./...` 均通過。
- 2026-07-29：`make verify` 通過 fmt-check、vet、golangci-lint v2.12.2、race、92.9% coverage 與 benchmark smoke；`git diff --check` 通過，未修改 workflow、module、dependency、Makefile 或公開 API。
- 2026-07-29：清除 `coverage.out` 後，`make clean` 的 `go clean` 因 sandbox 無權存取使用者 Go build cache 而結束；測試產物已移除，此環境限制不影響 T4 驗證結果。

## 後續改善

- [ ] 本 spec 與前置 CI T9 完成後，再建立 os.Root 原子 containment spec
