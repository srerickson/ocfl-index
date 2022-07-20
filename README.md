# ocfl-index

This is a command line tool for indexing [OCFL Storage Roots](https://ocfl.io). You can use `ocfl-index` to build and query a sqlite3-based index of the logical paths in an OCFL repository. See `sqlite/schema.sql` details on the data model. 

*This is work in progress*. 

```
Usage:
  ocfl-index [command]

Available Commands:
  benchmark   benchmarks indexing with generated inventories
  help        Help about any command
  index       index an OCFL storage root
  query       query the index

Flags:
  -f, --file string   index filename/connection string (default "index.sqlite")
  -h, --help          help for ocfl-index

Use "ocfl-index [command] --help" for more information about a command.
```

## Indexing

You can index OCFL storage roots stored on the local filesystem, on AWS S3, or in a zip archive. 

```sh
# index a storage root locally
ocfl-index index --dir ~/my/root

# index a storage root on s3
ocfl-index index --s3-bucket my-bucket --s3-path store-prefix
```

## Querying
To query, use the `query [object-id] [path]` subcommand:

```sh
# list all objects in the index
ocfl-index query

# list all versions in an object
ocfl-index query object-id

# list names in the root of an object (head version)
ocfl-index query object-id "."

# list name in the root of an object at a particular version
ocfl-index query -v v1 object-id "."
```

## S3 Credentials

Credentials set with the `aws` cli are used. You may also use the following environment variables:

```sh
# Access Key ID
AWS_ACCESS_KEY_ID= ... 
# Secret Access Key
AWS_SECRET_ACCESS_KEY=SECRET
# Region
AWS_REGION=us-east-1
```
