# 🐝 Busy Bee

**開會錄音 → AI 逐字稿 → 自動生成 PRD 與 Tech Spec** — 為開發團隊與技術主管打造的會議 AI 助手。

**Live**：https://busy-bee-502710.web.app（Google 登入，白名單制）

## 它做什麼

1. **錄音或上傳**會議音訊（瀏覽器 MediaRecorder / 拖曳上傳，支援 mp3 / m4a / webm / wav）
2. 背景管線自動處理：**Groq Whisper** 轉繁中逐字稿 → **Gemini** 生成結構化 **PRD** 與 **Tech Spec**
3. 處理狀態經 **WebSocket 即時推送**，完成後可全文**搜尋**歷史會議

## 架構亮點

```
React PWA ──(Firebase Hosting /api rewrite)──▶ Go API on Cloud Run（scale-to-zero）
    │                                              │ in-memory queue + worker（同 binary）
    └──(signed URL 直傳)──▶ GCS ◀──────────────────┤
                                                   ├──▶ Groq Whisper（STT）
         Neon PostgreSQL ◀─────────────────────────┤──▶ Gemini（PRD / Tech Spec）
```

幾個值得一提的設計決策（完整 ADR 見 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)）：

- **GCS Signed URL 直傳**（ADR-001）：音訊不經後端，繞過 Cloud Run 32MB 限制，後端零大檔記憶體壓力
- **無 Redis 的任務佇列**（ADR-010）：Postgres 是唯一真相源，in-memory 佇列 + 啟動/定期掃描復原。搭配**分階段冪等**（ADR-009），重啟重跑不重複扣 API 費用——這讓 scale-to-zero 成為可能，月費壓到 ~$0-2
- **WebSocket 首訊驗證**（ADR-002）：瀏覽器 WS 帶不了 Authorization header，改為連線後第一則訊息帶 Firebase JWT，驗證通過前零推送
- **零金鑰部署**：本地 GCS 簽名走 IAM impersonation、CI/CD 走 Workload Identity Federation——整條鏈路沒有任何落地的金鑰檔
- **Clean Architecture**：STT / LLM / 儲存 / 佇列全部是 domain port，換供應商只動 `infrastructure/` 一層

## 技術棧

| | |
|---|---|
| 後端 | Go 1.26 · Gin · sqlc + pgx（無 ORM）· slog |
| 前端 | React + Vite（PWA）· TypeScript · openapi-typescript 生成 API client |
| AI | Groq Whisper Large v3（STT）· Gemini Flash（文件生成） |
| 基礎設施 | Cloud Run · Firebase Hosting/Auth · GCS · Neon PostgreSQL · Secret Manager |
| CI/CD | GitHub Actions + WIF（測試含真實 Postgres 整合測試，main push 自動部署 + migration） |

## 本地開發

```bash
# 前置：Go 1.26+、Node 22+、Docker、ffmpeg
docker compose up -d                     # PostgreSQL

cd busy-bee-be
cp ../.env.example .env.local            # 填入各 key（見檔內註解）
go run ./cmd/migrate up
go run ./cmd/server                      # HTTP + worker，:8080

cd ../busy-bee-fe
npm install --legacy-peer-deps
npm run dev                              # :5173，/api 代理到後端
```

測試：`go test ./...`（整合測試自動使用本地 Docker Postgres；無環境時自動 skip）

## 文件

| 文件 | 內容 |
|------|------|
| [docs/PRODUCT.md](docs/PRODUCT.md) | 功能需求、驗收條件、Milestone |
| [docs/PLAN.md](docs/PLAN.md) | 開發計畫、Phase 進度、Session Log |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 架構設計、10+ 條 ADR 決策記錄 |

## License

[MIT](LICENSE)
