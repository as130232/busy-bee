# 🐝 Busy Bee

**錄音 / 上傳 / 貼連結 → AI 逐字稿（分講者）→ 依情境生成結構化摘要、行動項與可搜尋的知識庫**

**Live**：https://busy-bee-502710.web.app（Google 登入，白名單制）

## 它做什麼

輸入來源三選一：

1. **瀏覽器錄音**（MediaRecorder；錄音中持有 Screen Wake Lock，避免螢幕休眠中斷）
2. **上傳音訊**（拖曳上傳，支援 mp3 / m4a / webm / wav）
3. **貼連結匯入**（YouTube / Podcast / 直接音檔 URL，後端以 yt-dlp / HTTP 抓取音訊）

背景管線自動處理：

1. **Deepgram nova-2（zh-TW）** 轉繁中逐字稿，並做**語者辨識**（分講者 A/B/C，可自訂改名）
2. **Gemini** 依**情境模板**（會議 / 閒聊 / 面試）生成結構化摘要區塊，並抽取**行動項**（含到期日解析）
3. 逐字稿切塊 → **Gemini embedding** 存入 pgvector，供**語意搜尋**與**跨會議 RAG 問答**

處理狀態經 **WebSocket 即時推送**；完成後可播放音檔並**由摘要點擊跳到對應時間戳**。

## 功能

| 類別 | 功能 |
|---|---|
| 輸入 | 瀏覽器錄音、檔案上傳、貼連結匯入（YouTube / Podcast / 音檔） |
| 轉錄 | Deepgram 繁中逐字稿 + 語者辨識（分講者、可改名） |
| 生成 | 情境化結構化摘要（會議 / 閒聊 / 面試）、行動項抽取（到期日解析） |
| 搜尋 | 關鍵字 + 語意混合搜尋、跨會議 RAG 問答（答案帶 `[n]` 來源引用） |
| 整理 | 情境 / 來源篩選、自訂手動標籤、AI 自動標籤 |
| 提醒 | 會議行程提醒、行動項到期 **Web Push**（已打通 iOS）、`.ics` 加入行事曆 |
| 播放 | 音檔播放 + 摘要 ↔ 時間戳雙向跳轉 |
| 匯出 | 文件匯出與分享（分享連結匯入） |

資料一律 **owner-only**（僅本人可見）；播放 / 推播 / PWA 以 **iPhone 行動優先**設計。

## 架構亮點

```
React PWA ──(Firebase Hosting /api rewrite)──▶ Go API on Cloud Run（scale-to-zero）
    │                                              │ in-memory queue + worker（同 binary）
    └──(signed URL 直傳)──▶ GCS ◀──────────────────┤
                                                   ├──▶ Deepgram nova-2（STT + 語者辨識）
       Neon PostgreSQL + pgvector ◀───────────────┤──▶ Gemini（情境摘要 / 行動項 / 問答 / embedding）
                                                   ├──▶ yt-dlp / HTTP（貼連結匯入抓音訊）
                                                   └──▶ Web Push / VAPID（到期與行程提醒）
```

幾個值得一提的設計決策（完整 ADR 見 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)）：

- **GCS Signed URL 直傳**（ADR-001）：音訊不經後端，繞過 Cloud Run 32MB 限制，後端零大檔記憶體壓力
- **無 Redis 的任務佇列**（ADR-010）：Postgres 是唯一真相源，in-memory 佇列 + 啟動/定期掃描復原。搭配**分階段冪等**（ADR-009），重啟重跑不重複扣 API 費用——這讓 scale-to-zero 成為可能，月費壓到 ~$0-2
- **語者辨識選聲學分離**（ADR-011）：Deepgram nova-2 + `zh-TW`（nova-3 的 multi 對中文回空結果），一個聲音＝一位講者，較 LLM 推測式穩定
- **RAG 語意層**（ADR-006 升級）：逐字稿 embedding 存 pgvector，關鍵字（ILIKE）+ 語意（cosine top-K）混合搜尋；同一套檢索基建再支撐跨會議問答，問答無新增資料表、無相關片段不呼叫 LLM 的成本護欄
- **WebSocket 首訊驗證**（ADR-002）：瀏覽器 WS 帶不了 Authorization header，改為連線後第一則訊息帶 Firebase JWT，驗證通過前零推送
- **零金鑰部署**：本地 GCS 簽名走 IAM impersonation、CI/CD 走 Workload Identity Federation——整條鏈路沒有任何落地的金鑰檔
- **Clean Architecture**：STT / LLM / 儲存 / 佇列 / 匯入來源全部是 domain port，換供應商只動 `infrastructure/` 一層

## 技術棧

| | |
|---|---|
| 後端 | Go 1.26 · Gin · sqlc + pgx（無 ORM）· pgvector · slog |
| 前端 | React + Vite（PWA）· TypeScript · openapi-typescript 生成 API client |
| AI | Deepgram nova-2（STT + 語者辨識）· Gemini Flash（情境摘要 / 行動項 / 問答）· `gemini-embedding-001`（768 維語意向量） |
| 基礎設施 | Cloud Run（image 內含 yt-dlp）· Firebase Hosting/Auth · GCS · Neon PostgreSQL + pgvector · Secret Manager · Web Push（VAPID） |
| CI/CD | GitHub Actions + WIF（測試含真實 Postgres 整合測試，main push 自動部署 + migration） |

## 本地開發

```bash
# 前置：Go 1.26+、Node 22+、Docker、ffmpeg（貼連結匯入另需 yt-dlp）
docker compose up -d                     # PostgreSQL（pgvector image）

cd busy-bee-be
cp ../.env.example .env.local            # 填入各 key（見檔內註解）
go run ./cmd/migrate up
go run ./cmd/server                      # HTTP + worker，:8080

cd ../busy-bee-fe
npm install --legacy-peer-deps
npm run dev                              # :5173，/api 代理到後端
```

測試：

```bash
go test ./...                    # 單元測試（快、無外部依賴）
go test -tags integration ./...  # 含整合測試（需 Docker Postgres / GCS ADC / ffmpeg）
```

## 文件

| 文件 | 內容 |
|------|------|
| [docs/PRODUCT.md](docs/PRODUCT.md) | 功能需求、F-ID、驗收條件、Milestone |
| [docs/PLAN.md](docs/PLAN.md) | 開發計畫、Phase 進度、Session Log |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 架構設計、ADR 決策記錄、資料流 |

## License

[MIT](LICENSE)
