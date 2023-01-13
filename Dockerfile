FROM golang:1.19 as builder
WORKDIR /app
COPY . ./
RUN go mod download
RUN CGO_ENABLED=0 go build -tags=netgo -o ./ocfl-index ./cmd/ocfl-index

FROM cgr.dev/chainguard/static:latest
COPY --from=builder /app/ocfl-index /ocfl-index
ENV HOME /home/nonroot
ENV OCFL_INDEX_SQLITE /home/nonroot/index.sqlite
ENV OCFL_INDEX_STOREDIR /home/nonroot
CMD ["/ocfl-index", "server"]
