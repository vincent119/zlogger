# 設計文件：Go 1.25.12 安全修補

Status: Complete

## 決策

本次採最小 patch 升級，只調整 CI race matrix 與描述該現行基線的文件。`go.mod`
繼續宣告 Go 1.25.0，因為漏洞存在於執行掃描的標準庫工具鏈，不代表 library 語言
版本或 module 相容性門檻必須提高。

## 變更範圍

| 檔案 | 設計 |
|------|------|
| `.github/workflows/ci.yml` | race matrix 的 `1.25.11` 改為 `1.25.12` |
| `README.md` | 現行 Linux race 版本改為 Go 1.25.12／1.26.5 |
| `DESIGN.md` | 現行支援政策與 CI 表改為 Go 1.25.12／1.26.5 |
| 本規格 | 記錄需求、驗證與遠端驗收狀態 |

## 保護契約

- `DESIGN.md` 的既有 Go 1.25.11 benchmark 數據是歷史事實，維持不變。
- Go 1.26.5、runner、workflow event、timeout、Action SHA、Codecov、coverage、lint 與權限不變。
- 不變更 `go.mod`、`go.sum`、`.go`、`_test.go` 或阻塞中的依賴安全自動化規格。

## 驗證設計

1. 分別以 Go 1.25.12 與 Go 1.26.5 執行 race 測試。
2. 以 govulncheck v1.6.0 和官方即時資料庫掃描兩版工具鏈。
3. 在相同提交與硬體比較 Go 1.25.11／1.25.12 關鍵 logger benchmark。
4. 執行 `make verify`、YAML parse、`git diff --check` 與檔案邊界檢查。
5. push 後由遠端 CI 驗收兩版 race 與既有品質 job。

## 風險

- 官方漏洞資料庫會動態更新；驗證結果必須記錄日期、scanner 與工具鏈版本。
- 本機 benchmark 受系統雜訊影響；僅將一致且超過 10% 的變化視為需調查訊號。
- PR #18 已合併至 main；依賴安全自動化可改用 Go 1.25.12 恢復執行。
