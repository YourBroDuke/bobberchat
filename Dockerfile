FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bobberd ./cmd/bobberd
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bobber ./cmd/bobber
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bobber-tui ./cmd/bobber-tui

FROM alpine:3.19

WORKDIR /app
COPY --from=builder /out/bobberd /app/bobberd
COPY --from=builder /out/bobber /app/bobber
COPY --from=builder /out/bobber-tui /app/bobber-tui
COPY configs/backend.yaml /app/configs/backend.yaml

EXPOSE 8080
CMD ["/app/bobberd", "--config", "/app/configs/backend.yaml"]
