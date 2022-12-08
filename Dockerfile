FROM golang:1.19-alpine as builder
WORKDIR /app
COPY . ./

RUN go mod download
RUN go build -o ./ocfl-index ./cmd/ocfl-index

FROM alpine:latest
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app
COPY --from=builder /app/ocfl-index /app/ocfl-index

EXPOSE 8080

CMD ["/app/ocfl-index", "server", ":8080"]