.PHONY: install-devtools
install-devtools:
	# sqlc is used to generate code for sqlite queries
	go install github.com/kyleconroy/sqlc/cmd/sqlc@latest
	# buf is used for grpc code generation
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install github.com/bufbuild/connect-go/cmd/protoc-gen-connect-go@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest