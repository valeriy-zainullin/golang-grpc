#!/bin/bash

set -xe

mkdir -p gen

eval $(go env | grep GOPATH)
export PATH="$GOPATH"/bin:"$PATH"


if [ ! -d venv ]; then
  python3 -m venv venv
  venv/bin/pip3 install protoc-gen-swagger
fi
export PATH="$(pwd)/venv/bin":"$PATH"

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go get -u github.com/grpc-ecosystem/grpc-gateway/v2@v2.28.0
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

# --grpc-gateway_out=., оно само дописывает go-package. Не очевидно, конечно..
protoc \
  --go_out=./gen/ --go_opt=paths=source_relative \
  --go-grpc_out=./gen/ --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=. --grpc-gateway_opt generate_unbound_methods=true \
  --swagger_out=./gen/ \
  -I proto proto/blog.proto
