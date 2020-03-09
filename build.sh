#!/bin/sh

# This does not follow go-lang best practices, so we do some hackery here... but it will build the artifact
rm -rf ./bin
mkdir ./bin
go get github.com/Knetic/govaluate
go get github.com/op/go-logging
go get github.com/spf13/cobra
go get github.com/inconshreveable/mousetrap
go get gopkg.in/ini.v1
go get github.com/mattn/go-shellwords

VERSION=0.0.3

# windows
env GOOS=windows GOARCH=amd64 go build -o bin/eb ./main.go
zip -r bin/eb_${VERSION}_windows_amd64.zip bin/eb

# linux
env GOOS=linux GOARCH=amd64 go build -o bin/eb ./main.go
env GOOS=linux GOARCH=amd64 go build -o bin/eb-linux ./main.go
tar -czvf bin/eb_${VERSION}_linux_amd64.tar.gz bin/eb

# mac
env GOOS=darwin GOARCH=amd64 go build -o bin/eb ./main.go
tar -czvf bin/eb_${VERSION}_darwin_amd64.tar.gz bin/eb

echo "Finished compiling ./bin/eb"