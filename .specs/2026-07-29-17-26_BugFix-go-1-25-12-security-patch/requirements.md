# 需求文件：Go 1.25.12 安全修補

Status: Complete

## 背景

依賴安全自動化的基線掃描發現，CI 最低工具鏈 Go 1.25.11 會透過
`file_security.go` 的 `os.Root.OpenFile` 命中可達的 `GO-2026-4970`／
`CVE-2026-39822`。Go 官方公告指出此問題已在 Go 1.25.12 修正，因此必須先
更新最低 patch 版本，才能建立 fail-closed 的漏洞掃描閘門。

## 需求

### R1 更新最低 CI 工具鏈

- Race matrix 的最低版本由 Go 1.25.11 更新為 Go 1.25.12。
- 現行版本維持 Go 1.26.5。
- `go.mod` 的語言版本 `go 1.25.0` 不變。

### R2 同步現行文件

- README 與 DESIGN 的現行 CI 支援政策改為 Go 1.25.12／1.26.5。
- 既有 Go 1.25.11 benchmark 歷史數據不得改寫。

### R3 安全與相容性驗證

- Go 1.25.12 與 Go 1.26.5 的 `go test -race -count=1 ./...` 必須通過。
- govulncheck v1.6.0 使用官方 `https://vuln.go.dev` 掃描兩版工具鏈，皆不得有可達漏洞。
- Go 1.25.11 與 Go 1.25.12 的關鍵 benchmark 若顯示超過 10% 的明確差異，必須以
  聚焦重測確認影響範圍，並記錄是否接受的安全與效能風險決策。
- 專案既有完整驗證必須通過。

## 驗收條件

- [x] CI matrix 僅將 `1.25.11` 改為 `1.25.12`。
- [x] README 與 DESIGN 的現行政策一致。
- [x] 歷史 benchmark 數據、Go 1.26.5、Action SHA、job 結構與權限不變。
- [x] 兩版 race 與 govulncheck 通過。
- [x] 關鍵 benchmark 已完成調查與風險決策；`make verify`、YAML 與 diff 邊界檢查通過。
- [ ] 遠端 CI 通過後才可將本規格標記 Complete。

## 非目標

- 不修改產品碼、測試、公開 API、dependency、Action 或 `go.mod`／`go.sum`。
- 不在本分支實作 govulncheck CI、Dependabot 或 Makefile `vuln` target。
- 不改寫歷史規格與歷史 benchmark 數據。
