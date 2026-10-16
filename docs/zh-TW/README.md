# zlogger 使用說明

**繁體中文（台灣）** | [English](../en/README.md)

[專案首頁](../../README.md) · [GoDoc](https://pkg.go.dev/github.com/vincent119/zlogger)

第一次使用先看[設定](configuration.md)與[生命週期](lifecycle.md)。
只需要印出日誌的話，專案首頁的範例就能開始。

| 要處理的事 | 看這裡 |
| --- | --- |
| 選擇全域或獨立 logger，確認誰負責關閉 | [初始化與生命週期](lifecycle.md) |
| 設定預設值、解析設定檔、關掉終端機顏色 | [設定](configuration.md) |
| 每日三檔、自訂 sink、接收寫入故障 | [輸出模式](output-modes.md) |
| 自訂欄位名稱、傳遞 context、避免重複 key | [Context 與欄位](context-and-fields.md) |
| 啟用遮罩、檢查路徑與檔案權限 | [安全性](security.md) |
| 在應用端加入 HTTP 請求日誌 | [Gin 整合](integrations/gin.md) |
| 接上容量換檔、保留天數與壓縮 | [timberjack 整合](integrations/timberjack.md) |

這些文件對應目前分支。Gin 與 timberjack 範例由應用程式自行整合，
不代表 zlogger 內建這些依賴。欄位名稱和業務規則也由應用端決定。
