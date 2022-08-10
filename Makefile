.PHONY: startminio stopminio

startminio:
	##
	## checking podman install
	##
	which podman
	sudo podman pull quay.io/minio/minio:latest
	sudo podman run --name ocfl-test -d --rm -p 9000:9000 -p 9001:9001 -v $(shell pwd)/testdata/minio:/data:z  minio/minio server /data --console-address ":9001"

stopminio:
	##
	## stoping minio
	##
	which podman
	sudo podman stop ocfl-test
	