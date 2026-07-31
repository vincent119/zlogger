# zlogger 文件

**繁體中文** | [English](../en/README.md)

[返回專案首頁](../../README.md) | [GoDoc](https://pkg.go.dev/github.com/vincent119/zlogger)

## 建議閱讀順序

1. [設定](configuration.md)：設定來源、`ConfigPatch`、預設值與驗證。
2. [生命週期](lifecycle.md)：global、instance、cleanup、`Sync` 與 `Close`。
3. [輸出模式](output-modes.md)：console、file、每日分級與外部 sinks。
4. [Context 與 fields](context-and-fields.md)：request-scoped fields 與合併規則。
5. [安全性](security.md)：路徑、權限、`os.Root` 與敏感資料。
6. [Gin 整合](integrations/gin.md)：引用端 HTTP middleware 範例。
7. [timberjack 整合](integrations/timberjack.md)：容量、保留與壓縮 rotation。

根 README 提供快速開始與能力摘要；本目錄保存完整使用契約與進階整合說明。
