# 需求文件：以 os.Root 建立原子檔案 containment

## 來源

- Type：BugFix
- Owner：待確認
- Status：InProgress
- 前置規格：`.specs/2026-07-29-11-18_BugFix-file-output-security-boundary/`
- 前置基線：`.specs/2026-07-29-11-48_Chore-go-toolchain-ci-baseline/`

## 文件定位

本 spec 接續既有檔案輸出安全邊界。前置規格已完成 leaf 驗證、路徑 containment、既有最終 symlink 拒絕、`0700`／`0600` 權限與 `ErrUnsafeLogPath` 分類，但因當時最低版本限制，採用 `Lstat` 後再 `OpenFile`，兩次系統呼叫之間仍存在 TOCTOU race。

目前 module 最低版本已升至 Go 1.25，CI 也已在 Go 1.25.11、Go 1.26.5、macOS 15 與 Windows 2025 驗證，因此可使用標準庫 `os.Root`。本 spec 只替換檔案開啟 containment 機制，不重寫已完成的 Config、SplitOutput 生命週期、leaf 規則、權限或 redaction。

README 覆蓋率 badge 與 Codecov 上傳已由獨立 PR #7 恢復，不屬於本安全修正差異。

## 背景

目前 `openSecureLogFile` 的流程為：

1. 驗證 leaf 並以 `filepath.Rel` 確認字串路徑未逸出。
2. 對完整目標路徑執行 `os.Lstat`。
3. 若目標不是既有 symlink，呼叫 `os.OpenFile`。

攻擊者若能在步驟 2 與步驟 3 之間把 leaf 替換成指向 base 外的 symlink，`os.OpenFile` 可能跟隨新目標。`filepath.Rel` 只驗證字串路徑，無法約束檔案系統解析期間的並行變化。

`os.Root` 以已開啟的目錄作為操作根；其方法解析的檔名不可指向 root 外。它允許跟隨 root 內部 symlink，但 symlink 不得為絕對路徑，也不得解析至 root 外。

## 威脅模型

### 包含

- `LogPath`／SplitOutput `directory` 是呼叫端選定且可信任的 base directory。
- `FileName`／`filePrefix` 及其衍生日誌檔名是不可信任 leaf。
- 本機其他程序可能在檢查與開檔之間替換 leaf。
- 攻擊目標是讓一般或分級日誌寫入 base directory 外部。

### 不包含

- 呼叫端本身提供惡意或已遭竄改的 base directory。
- `os.OpenRoot` 執行前，可信任 base path 本身被替換或改指向其他位置。
- filesystem boundary、bind mount、`/proc`、Unix device file 或惡意 filesystem。
- Go `js`／`plan9`／`wasip1` 的平台限制。
- 自動敏感資訊遮罩、檔案加密或 log collector 權限管理。

## 目標

1. 一般 file output 的 leaf 解析與開啟由 `os.Root` 約束，不可因並行 symlink 替換寫到 base 外。
2. SplitOutput 每批 info、warn、error 檔案使用同一個已開啟 root，避免同批檔案解析到不同目錄實體。
3. 既有最終 symlink 在穩定狀態下仍回傳可由 `errors.Is(err, ErrUnsafeLogPath)` 判斷的錯誤。
4. root、已開啟檔案與 partial file set 的所有權明確，任何失敗路徑均關閉已取得資源。
5. root 的 `Close` 錯誤不得靜默忽略；錯誤需以 `errors.Join` 或 `%w` 保留。
6. 安全 leaf、空 FileName／prefix、日期檔名、分級檔名、append 與既有 mode 行為保持。
7. 一般與 SplitOutput 的公開 API、level routing、換檔及關閉契約不變。
8. README 與 DESIGN 改為描述 `os.Root` 已提供的 containment，以及仍不包含的威脅。
9. Go 1.25.11／1.26.5 race、macOS 15、Windows 2025、lint、coverage 與 benchmark 全部通過。

## 非目標

1. 不允許 nested relative path；leaf 驗證規則不放寬。
2. 不移除 `ErrUnsafeLogPath` 或改變 Config 的 `ErrInvalidConfig` error chain。
3. 不提供自訂 `DirPerm`／`FilePerm` options。
4. 不對既有檔案或目錄執行 chmod。
5. 不長期保存 `*os.Root` 於 `Instance` 或 `SplitOutput`；每次建立一批檔案時開啟並關閉 root。
6. 不新增 dependency、自製 syscall 或平台專用 `O_NOFOLLOW`。
7. 不修改 CI matrix、Go directive、Makefile 或 coverage 門檻。
8. 不處理 Context fields、encoder、SQL core 或效能重構。
9. 不在本 spec 恢復 Codecov 或新增 coverage badge。

## 現有行為

- leaf 字串 containment 已驗證。
- 穩定存在的最終 symlink 會在 `Lstat` 階段被拒絕。
- `Lstat` 後替換成外部 symlink 時，後續 `os.OpenFile` 仍可能跟隨。
- SplitOutput 的三個檔案分別呼叫 opener，沒有共享 directory handle。
- README 與 DESIGN 明確揭露 TOCTOU 剩餘風險。

## 新行為

- 所有日誌 leaf 都透過 `os.Root.OpenFile` 開啟。
- 每批檔案先 `os.OpenRoot(baseDir)`，完成所有 leaf 開啟後關閉 root。
- SplitOutput 同一輪換檔的三個 leaf 共用同一 root。
- 穩定 symlink 仍先以 `Root.Lstat` 拒絕；若在檢查後被替換，`Root.OpenFile` 仍保證解析結果不逸出 root。
- README 與 DESIGN 不再宣稱仍使用 `Lstat` 加完整路徑 `OpenFile`，並保留 base trust、mount boundary 與特殊平台限制。

## 使用情境

1. 使用者以一般 file output 寫入安全 leaf，檔案仍建立於 `LogPath` 並可追加。
2. 使用者以 SplitOutput 產生三個每日檔案，所有檔案都位於同一個 directory root。
3. 本機程序把目標 leaf 並行替換為指向 root 外的 symlink，logger 不得修改外部檔案。
4. 目標在開啟前已是 symlink，logger 維持既有拒絕與錯誤分類。
5. 建立第二或第三個 split 檔案失敗時，已開檔案與 root 都被關閉。

## 驗收情境

### 場景一：一般輸出拒絕穩定的外部 symlink

- 測試：`TestRootedFileOutputRejectsExistingSymlink`
- 假設：base 內的 `app.log` 是指向 base 外檔案的 symlink
- 當：建立一般 file output
- 那麼：建立失敗，錯誤符合 `ErrUnsafeLogPath`，外部內容不變

### 場景二：分級輸出拒絕穩定的外部 symlink

- 測試：`TestRootedSplitOutputRejectsExistingSymlink`
- 假設：warn leaf 是指向 base 外檔案的 symlink
- 當：建立 SplitOutput
- 那麼：建立失敗，info partial file 被關閉，外部內容不變

### 場景三：並行替換不得逸出 root

- 測試：`TestRootedFileOpenContainsConcurrentReplacement`
- 假設：另一個 goroutine 在一般 leaf 與外部 symlink 間反覆替換
- 當：測試重複執行開啟與寫入
- 那麼：外部 sentinel 內容始終不變；成功寫入只發生於 base 內

### 場景四：同批 SplitOutput 使用單一 root

- 測試：`TestOpenRootedLogFilesUsesSingleRoot`
- 假設：一批包含 info、warn、error 三個 leaf
- 當：批次 opener 建立三個檔案
- 那麼：只建立並關閉一個 root，三個檔案皆可用

### 場景五：partial failure 完整回收

- 測試：`TestOpenRootedLogFilesClosesResourcesOnFailure`
- 假設：第二或第三個 leaf 開啟失敗，或 root close 回傳錯誤
- 當：批次 opener 回傳
- 那麼：所有已開檔案與 root 均已關閉，錯誤原因完整保留

### 場景六：既有檔案契約保持

- 測試：`TestRootedFileOutputsPreserveExistingBehavior`
- 假設：安全 leaf 已存在且 mode 為 `0640`
- 當：一般與分級輸出追加內容
- 那麼：內容使用 append，既有 mode 不變，新資源仍為 private mode

### 場景七：換檔生命週期保持

- 測試：既有 `TestSplitOutputCloseStopsRotation` 與 rotation selectors
- 假設：SplitOutput 已啟動並觸發換檔
- 當：換檔成功、失敗或與 Close 交錯
- 那麼：既有檔案交易、goroutine 收斂、Close 冪等與錯誤行為不回歸

### 場景八：跨平台品質基線

- 測試：CI workflow 既有 jobs
- 假設：變更已 push
- 當：GitHub Actions 執行
- 那麼：Go 1.25.11／1.26.5 race、macOS 15、Windows 2025、lint、coverage 與 benchmark 全數通過

## 驗收條件

- 上述八個情境均有可追溯測試或 CI 證據。
- 產品碼不再以完整目標路徑呼叫 `os.OpenFile` 建立日誌檔。
- `os.Root.OpenFile` 是一般與分級輸出的共同開檔入口。
- 對 base 外 symlink 的並行替換不得改寫外部 sentinel。
- 所有 root 與 file ownership 路徑可由測試證明已關閉。
- `errors.Is` 對既有安全錯誤分類保持。
- 完整 `make verify` 通過且 coverage 不低於 90%。
- README、DESIGN 與實作的安全承諾一致。

## 驗證需求

- TDD：先保存目前實作在並行置換情境的 Red 證據，再導入 `os.Root`。
- 穩定性：安全與 rotation selectors 連續執行至少 20 次。
- Race：Go 1.25.11 與 Go 1.26.5 均執行 `go test -race -count=1 ./...`。
- 品質：`make verify`、`git diff --check`、邊界檢查。
- 遠端：macOS 15 與 Windows 2025 必須實際執行，不以本機推測替代。

## 待確認項目

- 無。
