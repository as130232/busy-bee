# --- build stage ---
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- runtime stage ---
# alpine（非 distroless）因為 worker 需要 ffmpeg 做音訊壓縮（Phase 6 起）
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata ffmpeg \
 && addgroup -S app && adduser -S app -G app

COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/migrate /usr/local/bin/migrate

USER app
EXPOSE 8080

ENTRYPOINT ["server"]
