.PHONY: install-devtools
install-devtools:
	# sqlc is used to generate code for sqlite queries
	go install github.com/kyleconroy/sqlc/cmd/sqlc@v1.17.2
	# buf is used for grpc code generation
	go install github.com/bufbuild/buf/cmd/buf@v1.15.1
	go install github.com/bufbuild/connect-go/cmd/protoc-gen-connect-go@v1.5.2
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.29.0