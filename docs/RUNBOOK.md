# Busy Bee 維運手冊

> 最後更新：2026-10-18
>
> 給處理線上問題的人看。系統設計與各依賴故障時的系統行為見 `docs/ARCHITECTURE.md`〈外部依賴〉〈部署與回滾〉；這裡只寫人要做什麼。

## 服務總覽

| 項目 | 值 |
|------|----|
| 前端 | https://busy-bee-502710.web.app（Firebase Hosting；`/api/**` 轉送 Cloud Run） |
| 後端 | Cloud Run `busy-bee-api`，region `asia-east1`，GCP 專案 `busy-bee-502710` |
| WebSocket | 前端直連 Cloud Run（`VITE_WS_BASE`），不經 Hosting |
| 健康檢查 | `GET /health`；版本 `GET /version`（回傳線上 commit） |
| 日誌 | Cloud Logging（Cloud Run 服務 `busy-bee-api`），JSON 結構化 log，可用 `traceId`、`meeting_id` 搜尋 |
| 部署 | push `main` 觸發 GitHub Actions（見 ARCHITECTURE〈部署與回滾〉） |
| 機敏設定 | Secret Manager（專案 `busy-bee-502710`），金鑰值不寫在文件 |

> 注意：本機 gcloud 預設專案可能不是 busy-bee，指令一律帶 `--project busy-bee-502710`。

## 告警處理

目前未設定自動告警（待確認，見 PLAN 29.12）。以下依「使用者回報的現象」處理：

### 錄音上傳後一直停在處理中 / 失敗

- **先查**：1. 該場 `meeting_id` 的 log（`meeting.process.*`）2. `/version` 是否為預期版本 3. Deepgram / Gemini 是否故障
- **常見原因與處理**：

  | 原因 | 判斷方式 | 處理 |
  |------|---------|------|
  | instance 剛被回收，任務中斷 | log 中斷、無錯誤 | 等下次喚醒由 Sweeper 自動續跑（ADR-010）；或請使用者在詳情頁按「重新處理」 |
  | Deepgram / Gemini 錯誤或額度用完 | log `process transcribe` / `llm.generate` 錯誤 | 確認供應商狀態與帳戶額度；恢復後按「重新處理」 |
  | 主模型下架 | log `llm.model_fallback` | 已自動備援（ADR-017）；評估更新 `GEMINI_MODEL` |

### 即時會議顯示「即時辨識已停用」

- **先查**：log 中 `/live/token` 的錯誤；`deepgram grant: status 403` 代表金鑰權限不足
- **處理**：Deepgram 金鑰須為 Member 以上權限，換金鑰步驟見下方〈輪換 Deepgram 金鑰〉。額度用完（`LiveQuotaExceeded`）屬預期行為，可調 `LIVE_MONTHLY_MINUTES_LIMIT`。

### 推播提醒沒收到

- **先查**：設定頁「發送測試通知」是否收到；log `push.*`
- **處理**：scale-to-zero 下提醒會延到 instance 喚醒才補發（2026-10-13 決策，PLAN 29.11），屬預期。測試通知也收不到 → 檢查 VAPID 設定（曾發生 `sub` 雙重 `mailto:`，見事故紀錄）。

### 前端出現「送出的內容太大」（41302）

- **先查**：哪個端點、body 多大（log `request failed` 帶 path）
- **處理**：全域上限 1MB（`interface/http/server.go` 的 `defaultBodyLimit`）；即時問答帶整場逐字稿約 12 萬字 ≈ 360KB，超過代表前端送了異常內容，先查前端而非直接放寬。

### 貼連結匯入 YouTube 失敗

- **先查**：log `fetch yt-dlp metadata` 的錯誤尾行（bot 偵測、extractor 過期）
- **處理**：image 內 yt-dlp 來自 alpine 套件（2026-10 實測 2025.11.12，已落後）；需改 pip 釘版並加 deno（PLAN 32.9）。指向內網的連結會回 `netguard` 錯誤，屬預期。

## 回滾

> ⚠️ 執行前先詢問使用者確認。

1. 找上一個正常的 revision：`gcloud run revisions list --service busy-bee-api --region asia-east1 --project busy-bee-502710`
2. ⚠️ 切流量：`gcloud run services update-traffic busy-bee-api --to-revisions <REVISION>=100 --region asia-east1 --project busy-bee-502710`
3. 前端：Firebase console → Hosting → 發佈紀錄 → 回滾到上一版（⚠️ 執行前先詢問）。
4. DB migration 一般不回退（都設計為向後相容）；必要時 ⚠️ `DB_URL=<取自 Secret Manager busy-bee-db-url> go run ./cmd/migrate down`（每次回退一版，執行前先詢問）。
5. 確認：`curl <後端網址>/version` 的 commit 為預期版本；實際錄一段短音檔走完流程。
6. 根因修好後照常 push `main` 重新部署（之後的部署會把流量切回最新 revision）。

## 輪換 Deepgram 金鑰

1. Deepgram console 建新金鑰：權限 **Member**、Expiration Never。
2. ⚠️ 新增 secret 版本：`printf '%s' '<新金鑰>' | gcloud secrets versions add busy-bee-deepgram-key --project busy-bee-502710 --data-file=-`
3. ⚠️ Cloud Run 改用新版本（目前以固定版本引用）：`gcloud run services update busy-bee-api --region asia-east1 --project busy-bee-502710 --update-secrets DEEPGRAM_API_KEY=busy-bee-deepgram-key:<新版本號>`
4. 確認一般錄音與即時會議都正常後，才到 Deepgram console 刪除舊金鑰。

## 外部依賴故障

各依賴故障時的系統行為見 ARCHITECTURE〈外部依賴〉。人要做的事：

| 依賴 | 人要做什麼 | 聯絡窗口 |
|------|-----------|---------|
| Neon（PostgreSQL） | 看 Neon console 狀態；恢復後未完成的處理由 Sweeper 續跑 | Neon status page |
| Deepgram | 看 status.deepgram.com 與帳戶額度；恢復後請使用者重新處理失敗的紀錄 | Deepgram console |
| Gemini | 看 AI Studio 額度與錯誤；主模型 404 會自動備援 | Google AI Studio |
| Firebase Auth / Hosting | 看 Firebase status；登入失敗時其他功能也無法使用 | Firebase console |
| GCS | 看 GCP status；上傳與播放失敗 | GCP console |

## 部署後確認

- `curl https://busy-bee-api-897794325314.asia-east1.run.app/version` 的 commit 與 main 一致
- 啟動 log 沒有 `config.insecure`（有則補設對應環境變數；`FRONTEND_ORIGINS` 應為 `busy-bee-502710.web.app,busy-bee-502710.firebaseapp.com`）
- GitHub Actions 的 `vulncheck-backend` job：紅燈不擋部署，但要評估是否升級依賴

## 常用排查指令

| 用途 | 指令 |
|------|------|
| 線上版本 | `curl -s https://busy-bee-api-897794325314.asia-east1.run.app/version` |
| 最近錯誤 log | `gcloud logging read 'resource.labels.service_name="busy-bee-api" AND severity>=ERROR' --project busy-bee-502710 --limit 50 --freshness 1d` |
| 單場會議 log | `gcloud logging read 'resource.labels.service_name="busy-bee-api" AND jsonPayload.meeting_id="<ID>"' --project busy-bee-502710 --limit 100` |
| revision 清單 | `gcloud run revisions list --service busy-bee-api --region asia-east1 --project busy-bee-502710` |
| CI 結果 | `gh run list --branch main --limit 5` |
| secret 版本 | `gcloud secrets versions list <SECRET> --project busy-bee-502710` |

## 事故紀錄

| 日期 | 現象 | 根因 | 處理 | 後續 |
|------|------|------|------|------|
| 2026-07-25 | iPhone 收不到推播；Apple 回 429 | VAPID JWT `sub` 雙重 `mailto:` 前綴；403 被當可重試導致殭屍訂閱狂送 | 修正前綴；403/404/410 一律清除訂閱 | 見 `docs/archive/PLAN-FOCUS-HISTORY.md` |
| 2026-09-30 | Cloud Run 月費偏高（159.5 TWD） | instance 計費 + 前端 WS 斷線後立即重連，instance 無法縮到 0 | WS 閒置斷線不重連 | 見 PLAN Session Log 2026-09-30 |
| 2026-10-15 | 即時會議無法簽發 Deepgram 短效金鑰（403） | 舊金鑰權限低於 Member | 建 Member 金鑰，secret 改用 `:2`（revision 00057） | PLAN 30.9 |
