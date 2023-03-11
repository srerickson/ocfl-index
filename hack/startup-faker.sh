# startup example
app="cmd/ocfl-index/ocfl-index"

# storage backend type: "fs", "s3", or "azure"
export OCFL_INDEX_BACKEND="azure"  

 # cloud bucket (for s3, azure)
export OCFL_INDEX_BUCKET="ocfl"

# path relative to bucket/fs to OCFL storage root
export OCFL_INDEX_STOREDIR="faker"

# path to index file
export OCFL_INDEX_SQLITE="faker.sqlite"

# number of go routines for object scan
export OCFL_INDEX_SCANWORKERS=100

# number of go routeins for inventory parse
export OCFL_INDEX_PARSEWORKERS=6


# Additional options:

# server port ("localhost:8080")
# export OCFL_INDEX_LISTEN

 # concurrency level (default to # of processors)
# export OCFL_INDEX_MAXGOROUTINES

# alternative s3 endpoint
# export AWS_S3_ENDPOINT

# start server and index filesizes
$($app server $@)
