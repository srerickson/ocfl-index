> **Note**
> This repo is for reference only. The code is no longer maintained. Work has moved to [https://github.com/srerickson/chaparral/](https://github.com/srerickson/chaparral/).

# ocfl-index

`ocfl-index` provides a lightweight http/gRPC-based API for indexing and accessing the contents of [OCFL-based repositories](https://ocfl.io). It can serve content from OCFL storage roots on the local file system or in the cloud (S3, Azure, and GCS). The index is currently stored in an sqlite3 database, however additional database backends may be implemented in the future.

This project is currently in a *pre-release* development phase. It should not be used in production settings and breaking changes to the API are likely.

This repository includes a command line client, `ox`, as well as protocol buffer schemata and service definitions that can be used to auto-generate client libraries for a variety of programming languages.

## Usage Example

### Configure and Start the Server
```sh
# s3 credentials
$ export AWS_ACCESS_KEY_ID= ... 
$ export AWS_SECRET_ACCESS_KEY=...
$ export AWS_REGION=...
$ export AWS_S3_ENDPOINT="http://localhost:9000" # for non-aws S3 endpoint

$ export OCFL_INDEX_BACKEND="s3"                # storage backend type: "fs", "s3", or "azure"
$ export OCFL_INDEX_BUCKET="ocfl"               # cloud bucket (for s3, azure)
$ export OCFL_INDEX_STOREDIR="public-data"      # path/prefix to storage root
$ export OCFL_INDEX_SQLITE="public-data.sqlite" # local path to index file

# start the server (see hack/startup_podman for container deployment example)
$ ocfl-index server
```

Alternatively, you can start the server with docker/podman:

```sh
# create volume for index data
volume="ocfl-index-data"
if ! $(podman volume exists $volume); then
    echo "creating volume: $volume"
    podman volume create "$volume"
fi

podman run --rm -it \
    -e AZURE_STORAGE_ACCOUNT="$AZURE_STORAGE_ACCOUNT" \
    -e AZURE_STORAGE_KEY="$AZURE_STORAGE_KEY" \
    -e OCFL_INDEX_BACKEND="azure" \
    -e OCFL_INDEX_BUCKET="ocfl" \
    -e OCFL_INDEX_STOREDIR="public-data" \
    -e OCFL_INDEX_SQLITE="/data/public-data.sqlite" \
    -v ocfl-index-data:/data \
    -p 8080:8080 \
    docker.io/srerickson/ocfl-index:latest
```

### Using the `ox` cli

```sh
# set server endpoint (default is "http://localhost:8080")
$ export OCFL_INDEX="https://myindex"

# build the index 
# the command returns immediately but the indexing process may take a while
$ ox reindex

# index status
$ ox status
> OCFL spec: 1.1
> storage root description: Demo Data Collections
> indexed inventories: 8

# list objects
$ ox ls
> 990041176260203776 v1 2022-10-10 21:30
> 990046797110203776 v1 2022-10-11 22:35
> ...

# list contents of an object 
$ ox ls 990041176260203776 
> [a1139d44] gazetteer.zip 
> [f2fb20b2] meta.json
> ...

# you can also reindex the object if it hasn't been
$ ox ls --reindex 990041176260203776
> [a1139d44] gazetteer.zip 
> [f2fb20b2] meta.json
> ...

# save object locally
$ ox export 990041176260203776 outdir
> downloading files ...

```

See the `clients` directory for gRPC client examples.

## API Documentation

The `ocfl-index` gRPC service definition is distributed using [buf.build](https://buf.build/srerickson/ocfl/docs/main:ocfl.v1#ocfl.v1.IndexService).

## Development

```sh
# sqlc is used to generate code for sqlite queries
go install github.com/kyleconroy/sqlc/cmd/sqlc@latest

# buf is used for grpc code generation
go install github.com/bufbuild/buf/cmd/buf@latest
go install github.com/bufbuild/connect-go/cmd/protoc-gen-connect-go@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

To regenerate gRPC stubs:

```sh
buf generate api
```