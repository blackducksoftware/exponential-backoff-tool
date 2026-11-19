#!/bin/sh

#
# Copyright 2020-present, Synopsys, Inc. * All rights reserved.
#
# This source code is licensed under the Apache-2.0 license found in
# the LICENSE file in the root directory of this source tree.

rm -rf ./bin
mkdir ./bin

# Dependencies are managed via go.mod / go.sum — no manual go get needed.

VERSION=0.0.7

# windows
env GOOS=windows GOARCH=amd64 go build -o bin/eb ./main.go
zip -r bin/eb_${VERSION}_windows_amd64.zip bin/eb

# mac
env GOOS=darwin GOARCH=amd64 go build -o bin/eb ./main.go
tar -czvf bin/eb_${VERSION}_darwin_amd64.tar.gz bin/eb

# linux
env GOOS=linux GOARCH=amd64 go build -o bin/eb ./main.go
tar -czvf bin/eb_${VERSION}_linux_amd64.tar.gz bin/eb


echo "Finished compiling ./bin/eb"
