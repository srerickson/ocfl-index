export OCFL_INDEX_BACKEND="azure"
export OCFL_INDEX_BUCKET="ocfl"
export OCFL_INDEX_STOREDIR="public-data"
export OCFL_INDEX_SQLITE="public-data.sqlite"
# export OCFL_INDEX_LISTEN
# export OCFL_INDEX_MAXGOROUTINES
# export AWS_S3_ENDPOINT

cmd/ocfl-index/ocfl-index server --filesizes