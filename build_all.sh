#!/usr/bin/env bash
set -euo pipefail

GOOS=linux GOARCH=amd64 go build \
    -o .build/getIE \
    -ldflags "-X main.BuildRev=$(git rev-parse --short HEAD)" \
    main.go

GOOS=windows GOARCH=amd64 go build \
    -o .build/getIE.exe \
    -ldflags "-X main.BuildRev=$(git rev-parse --short HEAD)" \
    main.go

GOOS=darwin GOARCH=amd64 go build \
    -o .build/getIE.app \
    -ldflags "-X main.BuildRev=$(git rev-parse --short HEAD)" \
    main.go

tar -czf getIE.tgz -C .build/ .
