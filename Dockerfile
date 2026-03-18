FROM golang:latest AS builder

WORKDIR /src
COPY go.work ./
COPY backend/go.mod backend/go.sum ./backend/
COPY cli/go.mod cli/go.sum ./cli/
RUN cd backend && go mod download
RUN cd cli && go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bobberd ./backend/cmd/bobberd
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/bobber ./cli/cmd/bobber

FROM alpine:3.19

WORKDIR /app
COPY --from=builder /out/bobberd /app/bobberd
COPY --from=builder /out/bobber /app/bobber
COPY configs/backend.yaml /app/configs/backend.yaml
COPY migrations/ /app/migrations/

EXPOSE 8080
CMD ["/app/bobberd", "--config", "/app/configs/backend.yaml"]
