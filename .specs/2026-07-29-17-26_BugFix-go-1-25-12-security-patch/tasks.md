# 任務文件：Go 1.25.12 安全修補

Status: InProgress

## Execution Context

- 分支：`security/go-1.25.12-patch`
- 根本原因：Go 1.25.11 的 `os.Root.OpenFile` 命中可達 `GO-2026-4970`
- 修復版本：Go 1.25.12
- 本輪授權：規格、實作與本機驗證；不含 commit、push 或遠端驗收
- Allowed Changes：`.github/workflows/ci.yml`、`README.md`、`DESIGN.md`、本規格
- Forbidden：`go.mod`、`go.sum`、產品碼、測試、Action SHA、既有 job 結構、阻塞中的自動化規格

## 任務

- [x] T0 建立安全修補規格
  - Status: Complete
  - Verify：需求、設計、邊界與停止條件已記錄

- [x] T1 更新現行 CI 與文件基線
  - Status: Complete
  - Depends：T0
  - Verify：CI、README、DESIGN 只更新現行最低 patch；歷史 benchmark 不變

- [x] T2 執行兩版 race 與漏洞掃描
  - Status: Complete
  - Depends：T1
  - Verify：Go 1.25.12／1.26.5 race 與 govulncheck v1.6.0 全數通過

- [x] T3 執行關鍵 benchmark 比較
  - Status: Complete
  - Depends：T1
  - Verify：Go 1.25.11／1.25.12 在相同環境比較；超過 10% 時聚焦重測並記錄風險決策

- [x] T4 執行完整驗證與邊界檢查
  - Status: Complete
  - Depends：T2、T3
  - Verify：`make verify`、YAML parse、diff check 與 Allowed Changes 通過

- [ ] T5 遠端 CI 驗收
  - Status: Planned
  - Depends：T4
  - Verify：使用者授權 push 後，PR 的既有七項 CI 全數通過

- [ ] T6 關閉規格並恢復自動化工作
  - Status: Planned
  - Depends：T5、合併至 main
  - Verify：本規格標記 Complete，依賴安全自動化規格改以 Go 1.25.12 繼續

## Implementation Notes

- 2026-07-29：依賴安全自動化基線以 govulncheck v1.6.0 發現 Go 1.25.11 可達
  `GO-2026-4970`／`CVE-2026-39822`；Go 官方資料指出 Go 1.25.12 已修正。
- 2026-07-29：從 main `7a7af29` 建立 `security/go-1.25.12-patch`；阻塞中的自動化規格
  保留為未追蹤檔案，不納入本分支變更。
- 2026-07-29：CI race matrix、README 與 DESIGN 的現行最低版本已由 Go 1.25.11
  更新為 Go 1.25.12；`DESIGN.md` 的 Go 1.25.11 歷史 benchmark 數據保持不變。
- 2026-07-29：Go 1.25.12／1.26.5 的 `go test -race -count=1 ./...` 均通過；
  govulncheck v1.6.0 分別辨識正確工具鏈，使用 `https://vuln.go.dev` 掃描皆回報
  `No vulnerabilities found.`。
- 2026-07-29：完整 logger benchmark 各 10 次的 geomean 為 +6.49%，B/op 與
  allocs/op 完全一致；主要 SplitOutput 三案例為 +0.8%、-1.5%、+1.7%，皆無統計
  顯著差異。初次量測因系統漂移有離群值，另做聚焦重測。
- 2026-07-29：聚焦 20 次重測確認 `WithContext` 單欄位建構由 71.24 ns/op 增至
  82.85 ns/op，+16.29% 且統計顯著，但絕對差異為 11.61 ns/op，B/op 與 allocs/op
  不變。實際 `InfoContext` 含欄位路徑為 785.5 對 763.6 ns/op，無統計顯著退化；
  Go 1.26.5 單欄位建構為 70.77 ns/op。基於可達高風險漏洞已確認、主要日誌路徑與
  現行工具鏈未退化，接受此 Go 1.25.12 限定的微型建構成本，不修改產品碼規避。
- 2026-07-29：`make verify` 通過，golangci-lint v2.12.2 為 0 issues，race 通過，
  atomic coverage 92.5%，benchmark smoke test 通過；相容 cache 重跑 lint 無警告。
- 2026-07-29：workflow YAML、`git diff --check`、Allowed Changes 與 go.mod／go.sum／
  `.go` 無差異檢查通過；遠端 CI 尚待使用者授權 commit／push。

## 驗證結果摘要

- 實作：本機範圍完成
- 本機驗證：通過；已記錄 Go 1.25.12 單欄位 context 建構的限定效能差異
- 遠端驗收：尚未授權
