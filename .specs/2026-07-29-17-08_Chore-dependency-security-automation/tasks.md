# 任務文件：依賴安全自動化

Status: Complete

## Execution Context

- 意圖：以固定 govulncheck、兩版 fail-closed CI 與限量 Dependabot 排程持續維護供應鏈安全
- 本輪授權：使用者已指示 `run task`，依本文件執行 Makefile、workflow、Dependabot、文件與本機驗證；commit、push 與遠端驗收需另行授權
- 非目標：不升級 dependency／Action／Go、不自動合併、不改 repository security setting、不新增 secret／permission、不修改產品碼
- 已定決策：govulncheck v1.6.0；官方即時 DB；Go 1.25.12／1.26.5 matrix；文字格式；make vuln 不進 verify；Dependabot weekly／錯峰／每 ecosystem limit 2；minor／patch 分組，major 獨立
- 邊界：實作限 Makefile、CI workflow、新 dependabot.yml、README、DESIGN、本 spec，以及 T8 明列的兩個歷史 checkbox
- 關鍵檔案：`Makefile`、`.github/workflows/ci.yml`、`.github/dependabot.yml`、`README.md`、`DESIGN.md`
- 完成條件：本機兩版掃描、失敗路徑、YAML／安全 contract、make verify、兩版 race、遠端九項 CI 與文件一致性全部通過

### Protected Behavior

- 現有七項 CI、workflow events、runner、Action SHA、Codecov、coverage、benchmark 與 lint 不變。
- permissions 維持 `contents: read`；不新增 secret、`pull_request_target` 或 `continue-on-error`。
- `make verify` 不新增外部網路依賴。
- go.mod、go.sum、產品碼、公開 API、測試與輸出行為不變。
- Dependabot PR 不自動合併，major 與 security update 不混入 minor／patch group。

### 邊界

#### Allowed Changes

- `Makefile`
- `.github/workflows/ci.yml`
- `.github/dependabot.yml`
- `README.md`
- `DESIGN.md`
- `.specs/2026-07-29-17-08_Chore-dependency-security-automation/`
- `.specs/2026-07-29-17-26_BugFix-go-1-25-12-security-patch/`，只回填 PR #18 遠端驗收與關閉狀態
- T8 僅可勾選 `.specs/2026-07-29-11-48_Chore-go-toolchain-ci-baseline/tasks.md` 的 govulncheck 與 Dependabot 後續項目

#### Forbidden

- 所有產品 `.go` 與 `_test.go` 檔
- `go.mod`、`go.sum`、runtime dependency、Go 版本與現有 Action SHA
- 修改或刪除現有七項 job、workflow event、coverage threshold、Codecov、benchmark 或 lint
- `@latest`、Action movable tag、額外 permissions、secret、`pull_request_target`、`continue-on-error`、`|| true`
- JSON／SARIF／OpenVEX gate、漏洞 suppression、DB snapshot／mirror、無上限 retry
- Dependabot auto-merge、reviewer、assignee、private registry 或 repository setting
- commit、push、release 或外部 repository mutation，除非使用者另行授權

## 任務依賴

| 任務 | Depends | 狀態 | 備註 |
|------|---------|------|------|
| T0 固定官方工具與 schema 契約 | 無 | Complete | v1.6.0 與 GitHub options 已由官方來源查核 |
| T1 記錄漏洞掃描與設定缺口基線 | T0 | Complete | PR #18 已升至 1.25.12，兩版掃描無漏洞 |
| T2 實作 Makefile vuln target | T1 | Complete | 缺工具、錯版本、成功三路徑通過 |
| T3 新增兩版 vulnerability CI | T2 | Complete | 總 job 數增為九項 |
| T4 新增 Dependabot weekly 設定 | T0 | Complete | gomod／github-actions 分開 |
| T5 更新 README 與 DESIGN | T2 至 T4 | Complete | scanner／DB／更新政策一致 |
| T6 本機完整驗證 | T1 至 T5 | Complete | 兩版 scan／race、verify、YAML 與邊界通過 |
| T7 遠端九項 CI 驗收 | T6 | Complete | run 30442557137 九項全數成功 |
| T8 回填完成狀態與歷史待辦 | T7 | Complete | 只更新明列 checkbox |

## 實作任務

- [x] T0 固定官方工具版本與 Dependabot schema
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：產品、CI、Makefile、Dependabot 與其他文件
  - Depends：無
  - Context：以 Go 官方 tag／pkg docs 確認 govulncheck v1.6.0、文字格式退出語意、`-db` 與 `-version`；以 GitHub 官方 docs 確認 version 2、gomod、github-actions、weekly、timezone、open-pull-requests-limit、groups、applies-to、update-types。記錄查詢日期與 URL。
  - Verify：
    - 所有版本與欄位有官方來源
    - 不使用搜尋摘要作唯一證據

- [x] T1 記錄現況與 vulnerability baseline
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes；暫存工具與輸出
    - Forbidden：repository 實作檔案、dependency 或持久環境設定
  - Depends：T0
  - Context：確認 Makefile 無 vuln、CI 無 vulnerability、dependabot.yml 不存在。將 v1.6.0 安裝至 `/private/tmp`，以兩版 Go 掃描 `./...`；記錄 scanner／Go／DB 與結果。若發現漏洞，停止實作並回報，不在本 spec 修復。
  - Verify：
    - `rg -n 'govulncheck|vulnerability|GOVULNCHECK' Makefile .github README.md DESIGN.md`
    - Go 1.25.12／1.26.5：`govulncheck -db=https://vuln.go.dev ./...`
    - `git status --short` 無工具或輸出產物

- [x] T2 實作 Makefile vuln target 與失敗契約
  - Status: Complete
  - Boundary:
    - Allowed Changes：`Makefile`、本 tasks Implementation Notes
    - Forbidden：workflow、Dependabot、產品碼、go.mod／go.sum
  - Depends：T1
  - Context：新增 GOVULNCHECK、GOVULNCHECK_VERSION、vuln phony 與 help。target 先檢查 binary，再以 `-version` 確認 `govulncheck@v1.6.0`，最後執行文字格式官方 DB 掃描。以暫存 fake binary 驗證錯版本；不加入 verify。
  - Verify：
    - 缺工具時 `make vuln` 非零並顯示固定安裝指令
    - fake 錯版本時掃描前非零
    - v1.6.0 正確工具 `make vuln` 通過
    - `make -n verify` 不含 vuln／govulncheck

- [x] T3 新增 Go 1.25.12／1.26.5 vulnerability CI matrix
  - Status: Complete
  - Boundary:
    - Allowed Changes：`.github/workflows/ci.yml`、本 tasks Implementation Notes
    - Forbidden：修改現有七項 job、Action SHA、event、permission、secret 或其他檔案
  - Depends：T2
  - Context：新增獨立 vulnerability job，ubuntu-24.04、15 分鐘、fail-fast false、兩版 matrix；沿用固定 checkout／setup-go SHA，安裝 `@v1.6.0`，顯示環境並執行 make vuln。不得使用 continue-on-error、結構化格式或 shell bypass。
  - Verify：
    - YAML parse 通過
    - job matrix、runner、timeout、SHA、install version 與 make target 符合 design
    - workflow 無新增 permission／secret／movable ref／`|| true`
    - 既有七項 job diff 只有新增 job 所需的共用 env 行

- [x] T4 新增 Dependabot weekly 更新政策
  - Status: Complete
  - Boundary:
    - Allowed Changes：`.github/dependabot.yml`、本 tasks Implementation Notes
    - Forbidden：workflow、repository setting、auto-merge、reviewer／assignee、private registry
  - Depends：T0
  - Context：新增 schema v2；gomod 週一 09:00、github-actions 09:30，Asia/Taipei，limit 2。各自建立 applies-to version-updates 的 minor／patch wildcard group；major 不 ignore、不分組。加 ecosystem 可辨識的 commit prefix。
  - Verify：
    - YAML parse 通過且兩個 updates entry 齊全
    - directory `/`、weekly、day、time、timezone、limit、groups、patterns、update-types 正確
    - 無 target-branch、ignore major、auto-merge、registries、reviewers、assignees 或 secrets

- [x] T5 更新 README 與 DESIGN
  - Status: Complete
  - Boundary:
    - Allowed Changes：`README.md`、`DESIGN.md`、本 tasks Implementation Notes
    - Forbidden：README.en、產品碼、未實作能力、scanner 結果捏造
  - Depends：T2 至 T4
  - Context：README 加固定安裝與 make vuln；DESIGN CI 表新增兩版 vulnerability job，說明 scanner 固定／DB 動態／fail closed／rerun，以及 Dependabot schedule、grouping、major/security review。不得宣稱 DB pinned、離線可用、自動修復或 auto-merge。
  - Verify：
    - `rg -n 'govulncheck|v1.6.0|vuln.go.dev|make vuln|Dependabot|Asia/Taipei|minor|patch|major' README.md DESIGN.md`
    - 文件與 Makefile／workflow／dependabot.yml 逐項一致

- [x] T6 本機完整驗證與邊界檢查
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過驗證擴張產品、dependency、CI 或 Dependabot scope
  - Depends：T1 至 T5
  - Context：執行兩版 vuln、兩版 race、make verify、YAML parse、安全字串與 diff boundary。確認 go.mod／go.sum、產品碼、七項既有 job、Action SHA、permission、secret 與產物不變。
  - Verify：
    - Go 1.25.12／1.26.5：`make vuln`、`go test -race -count=1 ./...`
    - `make verify`
    - workflow／dependabot YAML parse
    - `git diff --check`
    - `git diff --name-only` 只含 Allowed Changes
    - `git diff -- go.mod go.sum '*.go'` 無差異
    - `git status --short` 無 scanner／vulnerability／coverage 產物

- [x] T7 遠端九項 CI 驗收
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 tasks Implementation Notes
    - Forbidden：為通過 CI 降級 fail-closed、移除版本、加 bypass／permission／secret
  - Depends：T6
  - Context：經使用者另行授權 commit／push 後確認既有七項與兩版 vulnerability job 全部通過；job log 必須顯示 v1.6.0、對應 Go 版本及無可達漏洞。Dependabot 只有進入 default branch 後開始運作，不要求 PR 階段產生更新。
  - Verify：`gh pr checks`／`gh run view` 顯示九項 jobs pass，兩個 vulnerability logs 契約正確

- [x] T8 回填完成狀態與歷史待辦
  - Status: Complete
  - Boundary:
    - Allowed Changes：本 requirements／tasks；Execution Context 明列的兩個歷史 checkbox
    - Forbidden：其他 spec、產品碼或文件內容重寫
  - Depends：T7
  - Context：遠端 green 後將本 requirements／tasks 標記 Complete，附 PR、merge commit、run／job 證據；只勾選 CI 基線 spec 的 govulncheck 與 Dependabot 項目。Dependabot default branch 生效狀態若當下不可觀察，明確記錄限制，不捏造更新 PR。
  - Verify：文件狀態、checkbox 與 GitHub 證據一致

## 驗證任務

- [x] V1 govulncheck tool 與退出契約
  - v1.6.0 固定於 Makefile 與 workflow
  - 缺工具、錯版本、漏洞／DB error 均非零
  - 正確工具與無漏洞時兩版成功
  - 不使用永遠成功的結構化格式作 gate

- [x] V2 CI 供應鏈與最小權限
  - 新增兩版 matrix，總 job 數九項
  - 外部 Action 維持完整 SHA
  - permissions 只有 contents read
  - 無新 secret、pull_request_target、continue-on-error、movable ref 或 bypass

- [x] V3 Dependabot 更新契約
  - gomod／github-actions 分開 weekly 與錯峰
  - 每 ecosystem version PR limit 2
  - minor／patch grouped，major 與 security update 獨立
  - 無 auto-merge、private registry 或外部身份假設

- [x] V4 回歸、文件與邊界
  - 兩版 race、make verify、YAML parse、diff check 通過
  - go.mod／go.sum、產品碼與既有七項 CI 行為不變
  - README／DESIGN 不宣稱 DB pinned、離線、auto-fix 或 auto-merge
  - 暫存 scanner 與輸出不進 Git

## 實作中斷恢復

恢復時優先讀取：

1. 本文件的 `Execution Context`
2. 目前未完成 task
3. `Protected Behavior`
4. `Implementation Notes`

不得預設掃描整個 `.specs` 目錄。定位命令：

```bash
rg -n '^#|^##|^###|Boundary:|Depends:|Implementation Notes|Status:' .specs/2026-07-29-17-08_Chore-dependency-security-automation
```

## Implementation Notes

- 2026-07-29：從同步至 PR #17 merge commit `7a7af29` 的 main 建立分支 `chore/dependency-security-automation`。
- 2026-07-29：現況 CI 有七項 job，固定 Go 1.25.11／1.26.5、Action SHA、golangci-lint v2.12.2 與 `contents: read`；沒有 vulnerability job、Makefile vuln target 或 dependabot.yml。
- 2026-07-29：Go 官方來源確認 govulncheck 最新穩定 tag 為 v1.6.0；文字格式在發現漏洞時非零，JSON／SARIF／OpenVEX 不論 finding 都成功，因此本 spec 使用文字 gate。
- 2026-07-29：Go 官方文件確認 scanner 依 PATH 上的 Go build configuration 分析 source，故兩個支援 Go 版本分別掃描；`-db` 可指定 database，本 spec 明確使用官方即時 `https://vuln.go.dev`。
- 2026-07-29：GitHub 官方文件確認 dependabot.yml 位於 `.github/` default branch，schema version 2；gomod 與 github-actions 均受支援，weekly、timezone、PR limit 與 version/security groups 可設定。官方亦指出 version update limit 與 security update 內部上限分開。
- 2026-07-29：選擇 gomod／github-actions 分開排程與 group，不使用 multi-ecosystem group；minor／patch 分組，major 與 security update 維持獨立審查。
- 2026-07-29：requirements、design、tasks 已建立；依 SDD 流程停在規劃階段，尚未修改 Makefile、CI、Dependabot、README、DESIGN、歷史 checkbox 或執行掃描。
- 2026-07-29：使用者指示 `run task`，requirements 與 tasks 狀態改為 InProgress；T0 官方契約確認完成，從 T1 固定 scanner 與兩版漏洞基線開始執行。本批不含 commit、push 或遠端驗收。
- 2026-07-29：T1 將官方 `govulncheck@v1.6.0` 安裝至 `/private/tmp/zlogger-tools`；`-version` 顯示 Go 1.26.5、`Scanner: govulncheck@v1.6.0`、DB `https://vuln.go.dev`，確認 Makefile 可使用此精確輸出契約。
- 2026-07-29：Go 1.26.5 掃描 `./...` 回報 `No vulnerabilities found.`；Go 1.25.11 掃描以 exit code 3 阻擋，發現可達 `GO-2026-4970`／`CVE-2026-39822`，trace 為 `file_security.go:126` 的 `openRootedLogFilesWithPermissionsAndOpener` 呼叫 `os.Root.OpenFile`。
- 2026-07-29：Go 官方 vulnerability report 確認 `os.Root.OpenFile` 受影響範圍為 Go 1.25.12 之前及 Go 1.26.5 之前；Go release history 確認 1.25.12 於 2026-07-07 發布並包含 `os` security fixes。依本 spec Forbidden，不在此分支混入 Go／CI 升版，T1 與本 spec 標記 Blocked。
- 2026-07-29：PR #18 已以 merge commit `06d4524` 合併 Go 1.25.12 安全修補，七項
  CI 全數成功。重新以 govulncheck v1.6.0 和官方 DB 掃描 Go 1.25.12／1.26.5，
  兩版皆回報 `No vulnerabilities found.`；T1 完成，本 spec 解除 Blocked 並從 T2 恢復。
- 2026-07-29：T2 新增固定 `GOVULNCHECK_VERSION=1.6.0` 的 `make vuln`；缺工具與
  fake v0.0.0 均在掃描前非零退出，正確工具於兩版 Go 均通過，且 `make verify` dry-run
  不含 vuln target。
- 2026-07-29：T3 新增 Go 1.25.12／1.26.5 vulnerability matrix，沿用完整 checkout／
  setup-go SHA、ubuntu-24.04、15 分鐘與 `contents: read`，未新增 secret 或 bypass。
- 2026-07-29：T4 新增 gomod／github-actions weekly Dependabot 設定，Asia/Taipei
  09:00／09:30、各 limit 2、minor／patch version group、major 與 security 獨立。
- 2026-07-29：T5 已同步 README 與 DESIGN；兩份 YAML 均可解析，靜態 contract 與
  `git diff --check` 通過。Dependabot 是否由 GitHub 接受仍待 default branch 後觀察。
- 2026-07-29：T6 的 Go 1.25.12／1.26.5 `go test -race -count=1 ./...` 均通過；
  `make verify` 通過，包含 golangci-lint v2.12.2 的 0 issues、race、92.5% atomic
  coverage 與 benchmark smoke test。
- 2026-07-29：兩份 YAML parse、預期九項 checks、14 個 Action refs 全為 40 字元 SHA、
  禁止字串、`git diff --check` 與 Allowed Changes 均通過；go.mod、go.sum、產品與測試
  `.go` 無差異，coverage 產物已清理。
- 2026-07-29：PR #19 已以 head `b37dfa7`、merge commit `99d5668` 合併至 main；
  GitHub Actions run `30442557137` 的九項 checks 全數成功。
- 2026-07-29：vulnerability job `90545057899` 顯示 Go 1.25.12、govulncheck v1.6.0、
  DB `https://vuln.go.dev` 與 `No vulnerabilities found.`；job `90545057970` 對
  Go 1.26.5 顯示相同 scanner、DB 與無漏洞結果。
- 2026-07-29：`.github/dependabot.yml` 已隨 merge commit 進入 default branch；GitHub
  沒有在本次驗收提供可證明首次排程已執行的同步狀態，且規格不要求立即產生更新 PR，
  因此只記錄設定已生效的前置條件，不捏造排程或更新 PR 結果。
- 2026-07-29：T7、T8、V1 至 V4 與本 spec 狀態關閉；CI 基線 spec 只勾選
  govulncheck 與 Dependabot 兩個明列後續項目。

## 驗證結果摘要

- 新行為驗證：本機與遠端通過；兩版 vulnerability job 使用正確工具鏈、scanner 與 DB
- 回歸驗證：遠端九項 checks 全數成功；本機 coverage 92.5%、lint 0 issues
- 文件一致性：README、DESIGN、Makefile、workflow、Dependabot 與 SDD 契約一致
- 剩餘風險：官方 DB 可用性與即時資料漂移；Dependabot 首次排程尚待 GitHub 後續執行

## 後續改善

- [ ] 評估 SARIF／Code Scanning 整合所需 permissions 與 finding exit policy
- [ ] 累積 DB outage 資料後再評估有上限 retry，不預先加入複雜度
- [ ] repository 啟用 Dependabot alerts／security updates 時，記錄 owner 與處理 SLA
