# ocfl-index

`ocfl-index` provides a lightweight http/gRPC-based API for indexing and accessing the contents of [OCFL-based repositories](https://ocfl.io). It can serve content from OCFL storage roots on the local file system or in the cloud (S3, Azure, and GCS). The index is currently stored in an sqlite3 database, however additional database backends may be implemented in the future.

This project is currently in a *pre-release* development phase. It should not be used in production settings and breaking changes to the API are likely.

This repository includes a command line client, `ox`, as well as protocol buffer schemata and service definitions that can be used to auto-generate client libraries for a variety of programming languages.

## Usage Example

```sh
# server config
# S3 access settings
export AWS_ACCESS_KEY_ID= ... 
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=...
export AWS_S3_ENDPOINT="http://localhost:9000" # for non-aws S3 endpoint

# Azure is also supported
# export AZURE_STORAGE_ACCOUNT=...
# export AZURE_STORAGE_KEY=...

# storage backend type: "fs", "s3", or "azure"
export OCFL_INDEX_BACKEND="s3"  
# cloud bucket (for s3, azure)
export OCFL_INDEX_BUCKET="ocfl"
# path relative to bucket/fs to OCFL storage root
export OCFL_INDEX_STOREDIR="public-data"
# path to index file
export OCFL_INDEX_SQLITE="public-data.sqlite"

# Additional options (defaults shown)
# export OCFL_INDEX_LISTEN="localhost:8080"
# export AWS_S3_ENDPOINT = ""

# start the server
ocfl-index server
```

```sh
# use curl to get index summary
curl --header "Content-Type: application/json" http://localhost:8080/ocfl.v0.IndexService/GetSummary --data '{}'
```

## Development

```sh
# sqlc is used to generate code for sqlite queries
go install github.com/kyleconroy/sqlc/cmd/sqlc@latest

# buf is used for grpc code generation
go install github.com/bufbuild/buf/cmd/buf@latest
go install github.com/bufbuild/connect-go/cmd/protoc-gen-connect-go@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```


To regenerate gRPS stubs:

```sh
buf generate api
```