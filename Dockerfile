FROM golang:1.19-alpine as builder
WORKDIR /app
COPY . ./
RUN go mod download
RUN go build -o ./ocfl-index ./cmd/ocfl-index

FROM alpine:latest
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
RUN mkdir /data
RUN mkdir /repo

COPY --from=builder /app/ocfl-index /app/ocfl-index

EXPOSE 8080
ENV OCFL_INDEX_SQLITE="/data/index.sqlite"
ENV OCFL_INDEX_STOREDIR="/repo"

CMD ["/app/ocfl-index", "server", ":8080"]