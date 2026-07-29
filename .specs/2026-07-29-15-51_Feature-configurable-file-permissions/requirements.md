# 需求文件：可配置檔案輸出權限

## 來源

- Draft: 無
- Type: Feature
- Owner: vincent119
- Status: Complete

## 文件定位

本 spec 接續 `.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/` 的後續改善「評估可配置 DirPerm／FilePerm functional options」。只新增一般檔案輸出與分級輸出的建立權限選項，不重寫已完成的 leaf validation、`os.Root` containment、rotation lifecycle、Config 初始化契約或 encoder 行為。

參考來源：

- 需求來源：檔案輸出安全規格的後續改善項目
- 既有文件：`README.md` 的「檔案輸出安全」、`DESIGN.md` 的檔案輸出安全邊界
- 既有程式碼：`file_security.go`、`core.go`、`split_output.go`

## 背景

目前新建日誌目錄固定使用 `0700`，新建日誌檔案固定使用 `0600`。這是安全預設值，但需要以群組帳號讀取日誌的部署環境無法在建立階段要求 `0750`／`0640`；呼叫端只能在事後自行調整權限，容易產生短暫權限不一致與額外競態。

## 問題陳述

檔案輸出沒有安全且一致的建立權限選項。直接把 mode 加入 `Config` 會改動設定 schema 與非具名 struct literal，相反地，直接把既有建構函式改成 variadic 也會破壞函式值型別相容性。

## 目標

1. 提供共用 functional options，讓一般與分級輸出可指定新目錄及新檔案的 POSIX permission bits。
2. 保留所有既有公開函式簽章及 `Config` schema，既有呼叫仍使用 `0700`／`0600`。
3. 拒絕型別位元、缺少 owner 必要權限或 other-write 的不安全 mode。
4. 明確保留 umask、既有目錄／檔案權限不變及 Windows 語意限制。

## 非目標

1. 不對既有目錄或檔案執行 `chmod`。
2. 不提供 owner、group、ACL、Windows DACL 或 Kubernetes `fsGroup` 管理。
3. 不修改 leaf validation、`os.Root` containment、檔名格式、rotation 或 level routing。
4. 不變更 `Config`／`ConfigPatch` 欄位與設定檔 schema。

## 已定決策

- 既有 `New`、`Configure`、`NewSplitOutput`、`GetSplitCore` 簽章不變，另增對應 `*WithOptions` 入口。
- 使用共用 `FileOutputOption`、`WithDirPerm`、`WithFilePerm`。
- 預設 mode 維持目錄 `0700`、檔案 `0600`。
- mode 只影響新建立物件，仍受 process umask 限縮；既有 mode 不改寫。
- 允許 group read／write 與 other read／execute，但拒絕 other-write；目錄必須包含 owner `rwx`，檔案必須包含 owner `rw`。
- 無效 option 統一保留可由 `errors.Is` 判斷的 `ErrInvalidFilePermission`。

## 待確認項目

- 無。

## 現有行為

- `New`／`Configure` 建立新目錄時要求 `0700`，建立新檔案時要求 `0600`。
- `NewSplitOutput`／`GetSplitCore` 使用相同固定值，且每日換檔也固定為 `0600`。
- process umask 可進一步移除權限；既有物件權限維持不變。

## 新行為

- 既有入口行為完全不變。
- 呼叫端可使用 `NewWithOptions`／`ConfigureWithOptions` 指定一般檔案輸出權限。
- 呼叫端可使用 `NewSplitOutputWithOptions`／`GetSplitCoreWithOptions` 指定分級輸出權限，且每日換檔沿用同一組設定。
- 無效 mode 在任何檔案 I/O 前回傳 `ErrInvalidFilePermission`。

## 影響範圍

- 使用者：需要以群組帳號讀取或共同寫入日誌的 Go library 使用者
- 功能：一般檔案輸出、分級檔案輸出、每日換檔
- API / CLI：新增四個 `*WithOptions` 入口、`FileOutputOption`、兩個 option constructor 與錯誤 sentinel；既有 API 不變
- Data / Storage：只影響新建立目錄與檔案的 permission bits
- 文件 / 安裝 / 發布：更新 README、DESIGN 與 godoc；不新增 dependency

## 使用情境

- 作為以共用 Unix group 收集日誌的服務維運者，我想在建立日誌時指定 `0750`／`0640`，以便 sidecar 或群組成員可讀取日誌而不開放其他使用者寫入。

## 驗收情境

### 情境：一般輸出使用自訂權限

- 場景：新建一般日誌目錄與檔案
- 測試：`TestNewWithOptionsUsesConfiguredPermissions`
- 假設：非 Windows、目標目錄不存在，測試暫時將 umask 設為 `0000` 並可靠還原
- 當：以 `WithDirPerm(0o750)`、`WithFilePerm(0o640)` 建立 file output
- 那麼：新目錄 mode 為 `0750`，新檔案 mode 為 `0640`

### 情境：分級輸出與換檔沿用自訂權限

- 場景：建立三個分級檔案並觸發每日換檔
- 測試：`TestSplitOutputWithOptionsPreservesPermissionsAcrossRotation`
- 假設：非 Windows、使用可控制 clock，建立權限為 `0750`／`0640`
- 當：建立 `SplitOutput` 並觸發下一日 rotation
- 那麼：初始與換檔後的新檔案都要求 `0640`，目錄要求 `0750`

### 情境：無效權限在 I/O 前被拒絕

- 場景：傳入型別位元、缺少 owner 必要權限或 other-write
- 測試：`TestFileOutputOptionsRejectInvalidPermissions`
- 假設：目標路徑尚不存在
- 當：任一 `*WithOptions` 入口解析無效 option
- 那麼：回傳可由 `errors.Is` 判斷的 `ErrInvalidFilePermission`，且不建立目錄或檔案

### 情境：既有 API 與安全預設值不被破壞

- 場景：不提供 options 的一般與分級輸出
- 測試：`TestFileOutputsUsePrivatePermissions`
- 假設：使用既有 `New` 與 `NewSplitOutput`
- 當：建立新的日誌目錄與檔案
- 那麼：仍使用 `0700`／`0600`，既有公開函式簽章保持不變

### 情境：既有權限不被改寫

- 場景：目標目錄或日誌檔案已存在
- 測試：`TestFileOutputOptionsPreserveExistingPermissions`
- 假設：既有物件 mode 與 option 指定值不同
- 當：使用自訂 options 開啟輸出
- 那麼：既有物件 mode 維持不變，檔案仍以 append 開啟

## 驗收條件

1. 五個驗收情境均有可重複測試，Unix mode 測試在 Windows 只跳過 POSIX mode assertion，不跳過 option validation 與 lifecycle。
2. 既有四個公開函式簽章與 `Config`／`ConfigPatch` 結構不變。
3. 所有新入口在 I/O 前解析 options，無效 mode 以 `ErrInvalidFilePermission` 回傳。
4. `go test -race -count=1 ./...`、目標測試連續 20 次、`make verify` 與 `git diff --check` 通過。
5. README 與 DESIGN 明列 umask、既有 mode 不變、Windows 限制及放寬權限的責任邊界。

## 驗證需求

- Unit / Integration：`go test -race -count=1 -run 'Test(FileOutputOptions|NewWithOptions|SplitOutputWithOptions|FileOutputsUsePrivatePermissions)' ./...`
- CLI / Dry-run：無
- 文件檢查：核對 `README.md`、`DESIGN.md` 與 exported godoc 的預設值及限制一致
- 回歸驗證：`go test -race -count=1 ./...`、`make verify`

## 風險與假設

| 類型 | 內容 | 處理方式 |
|------|------|----------|
| 風險 | process umask 使實際 mode 比要求值更嚴格 | 文件明確說明；測試在受控區段暫設 umask 並立即還原 |
| 風險 | 測試修改 process umask 會影響並行測試 | mode 精確值測試不得使用 `t.Parallel`，以單一 helper 配對設定與還原 |
| 風險 | options 未保存至 rotation 狀態 | `SplitOutput` 保存解析後不可變設定，rotation 測試檢查第二日檔案 |
| 風險 | 放寬 mode 暴露日誌內容 | 預設不變、拒絕 other-write，README 警告呼叫端評估群組與敏感資料 |
| 假設 | Unix `MkdirAll`／`OpenFile` 建立 mode 仍受 umask 限縮 | 以標準庫契約與跨平台 CI 驗證 |

## 摘要

- 關鍵決策：新增相容的 `*WithOptions` API，不變更既有簽章或 Config schema
- 待確認項目：無
- 風險：umask、rotation 設定傳遞與權限放寬
- 下一步：依 `tasks.md` 先建立 TDD Red tests，再實作共用 option 解析
