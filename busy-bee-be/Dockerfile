# --- build stage ---
# --platform=$BUILDPLATFORM：編譯跑在建置機原生架構（Apple Silicon 上為 arm64），
# 以 GOARCH 交叉編譯目標架構——避免 QEMU 模擬 Go 編譯器（會 panic）。
# 基底映像釘 digest（多架構 index），避免同一個 tag 被換內容；由 Dependabot 開 PR 更新。
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
ARG TARGETARCH
# 建置資訊注入 /version（GIT_SHA=commit 短碼、BUILT_AT=建置時間 UTC）；未傳時保留程式預設 dev/unknown。
ARG GIT_SHA=dev
ARG BUILT_AT=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags="-s -w -X main.commit=$GIT_SHA -X main.builtAt=$BUILT_AT" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- runtime stage ---
# alpine（非 distroless）因為 worker 需要 ffmpeg 做音訊壓縮（Phase 6 起）
# yt-dlp：貼連結匯入從 YouTube 等站台抽音訊（Phase 21）
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache ca-certificates tzdata ffmpeg yt-dlp \
 && addgroup -S app && adduser -S app -G app

COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/migrate /usr/local/bin/migrate

USER app
EXPOSE 8080

ENTRYPOINT ["server"]
