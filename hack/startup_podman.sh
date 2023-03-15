#!/usr/bin/env bash

# volume for index data
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