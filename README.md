# ocfl-index

`ocfl-index` is a command line tool for indexing [OCFL Storage Roots](https://ocfl.io). It supports access to the [logical](https://ocfl.io/1.0/spec/#dfn-logical-state) directory structure of OCFL Objects. The index is stored as a sqlite3 database (see `sqlite/schema.sql` for details). This is an experimental project and the command line interface should not be considered stable.

```
Usage:
  ocfl-index [command]

Available Commands:
  benchmark   benchmark indexing with generated inventories
  help        Help about any command
  index       index an OCFL storage root
  query       query the index

Flags:
  -f, --file string   index filename/connection string (default "index.sqlite")
  -h, --help          help for ocfl-index

Use "ocfl-index [command] --help" for more information about a command.
```

## Indexing

OCFL storage roots can be read from the local filesystem, S3 buckets, or Azure Blob containers.

```sh
# index a storage root locally
ocfl-index index --path ~/my/root

# index a storage root in an S3 bucket
ocfl-index index --driver s3 --bucket my-bucket

# index a storage root in an S3 bucket with a prefix
ocfl-index index --driver s3 --bucket my-bucket --path my-prefix

# index a storage root in an Azure Blob container
ocfl-index index --driver azure --bucket my-container

```

## Querying
To query, use the `query [object-id] [path]` subcommand. The path should be a *relative* path (using `/` as a separator) referencing a file or directory in the object. Use the `-v` flag to query the object at a particular version.

```sh
# list all objects in the index
ocfl-index query

# list all versions in an object
ocfl-index query object-id

# list names of files and directories in the root of an object's most recent version
ocfl-index query object-id "."

# list names in the 'foo' directory of the object's first version
ocfl-index query object-id "foo" -v v1
```

## Benchmarking

The benchmark command can be used to get a sense of the performance characteristics of the index. It uses generated inventories with randomized states to build the index, measuring average times for index and query operations. It’s also useful for getting a sense of how the index file grows in size as you add inventories. 

```sh
# example with 1000 inventories
ocfl-index benchmark --size 100 --num 1000

indexing 1000 generated inventories (1-4 versions, 100 files/version)
indexed 1000/1000 (0.16 sec/op avg)
queried 99 paths (0.0004 sec/op avg)
benchmark complete in 164.5 sec
```

## S3 & Azure Config

Use environment variables to configure access settings for S3 and Azure:

```sh
# S3 access settings
export AWS_ACCESS_KEY_ID= ... 
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=...
export AWS_S3_ENDPOINT="http://localhost:9000" # for non-aws S3 endpoint

# Azure access settings
 export AZURE_STORAGE_ACCOUNT=...
 export AZURE_STORAGE_KEY=...
```
