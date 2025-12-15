# Commit Message 規範（Conventional Commits 中文版）

請在產生 commit message 時，完整遵循以下格式與規則。

---

## 🔥 Commit Message 格式

所有提交必須符合以下格式：

### 說明

- **type**：必填，表示此次修改的類型
- **scope**：可選，表示影響的範圍（模組名稱、功能區域等）
- **summary**：必填，簡要描述本次修改內容（需一句話概述，避免太長）

---

## 🧩 可用的 type 類型（中文說明）

| type     | 說明                                         |
| -------- | -------------------------------------------- |
| feat     | 新增功能                                     |
| fix      | 修復 Bug                                     |
| docs     | 文件新增、修改、更新                         |
| style    | 程式碼格式調整（不影響邏輯，例如排版、lint） |
| refactor | 重構（不新增功能、不修 Bug）                 |
| perf     | 性能優化                                     |
| test     | 測試新增或修改                               |
| build    | 建置系統、CI、CD 修改                        |
| chore    | 其他不影響程式邏輯的雜項修改                 |
| revert   | 回滾前一次 commit                            |
| ci       | CI pipeline / workflow 調整                  |
| config   | 設定檔修改（環境變數、config、yaml 等）      |

---

## 🎯 scope 使用建議（可選，但鼓勵使用）

scope 應簡短，通常為：

- 模組名稱：`auth`、`user`、`payment`
- 技術元件：`logging`、`api`、`db`、`router`
- 程式語言層：`handler`、`middleware`、`service`
- 其他自訂模組

範例：

```bash
eat(logging): 新增 ConsoleSeparator 支援
fix(api): 修正 session 驗證邏輯
refactor(service): 優化用戶查詢流程

```

---

## 📝 summary 撰寫規範

summary 必須：

- 使用 **動詞開頭**（新增、修正、調整、重構…）
- 不加句號（`.`）結尾
- 清晰簡短（建議不超過 70 字元）
- 不可模糊（例如「update code」這類不接受）

### ✔ 正確示例

```bash

feat(auth): 新增 MFA Session 驗證中介層
fix(logging): 修正 context 未攜帶 traceID 的問題
refactor(db): 調整查詢邏輯以降低延遲
```

### ✖ 錯誤示例（禁止）

```bash

update code
fix bug
修改
調整一些東西
```

---

## 🚫 不允許的 Commit Message

- 無 type 或格式錯誤
- type 不是標準類型
- summary 過長或模糊
- 未使用動詞
- 用中文全句當 type（例如「修正」、「更新」）

---

## 🤖 AI（Cursor）產生 commit 時必須遵守

1. 必須使用 **Conventional Commits 格式**
2. 一定要有 **type**
3. scope 有則使用，無則省略，但不可亂寫
4. summary 必須使用中文
5. summary 必須保持簡短（< 70 字）
6. 不可生成模糊訊息（如 update、fix bug）
7. 若涉及多項變更，AI 必須挑選「最主要目的」作為 summary
8. 不得生成破壞格式的內容

---

## 🧪 實際範例

```bash
feat(logging): 新增 ConsoleSeparator 以提升日誌可讀性
fix(api): 修正 X-MFA-Session 標頭缺失時的驗證錯誤
refactor(handler): 重構 Webhook 處理流程以改善維護性
perf(db): 優化查詢索引以降低延遲
chore(config): 更新 Docker Compose 預設埠號
test(auth): 新增 MFA Session 驗證單元測試
```

---

本規範適用於 Cursor AI 在本專案生成的所有 commit message。
