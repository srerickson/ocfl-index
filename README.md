# ocfl-index

This is a command line tool for indexing [OCFL Storage Roots](https://ocfl.io). You can use `ocfl-index` it to build and query a sqlite3-based index of the logical paths in an OCFL repository. See `sqlite/schema.sql` details on the data model.

## Indexing

You can index OCFL storage roots stored on the local filesystem, on AWS S3, or in a zip archive. 


## S3 Credentials

Credentials set with the `aws` cli are used. You may also use the following environment variables:

```sh
# Access Key ID
AWS_ACCESS_KEY_ID= ... 
# Secret Access Key
AWS_SECRET_ACCESS_KEY=SECRET
# Refion
AWS_REGION=us-east-1
```