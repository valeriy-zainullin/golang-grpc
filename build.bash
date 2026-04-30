#!/bin/bash 

set -xe

go build .

go test -c -o main-test ./services
