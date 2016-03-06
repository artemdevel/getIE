#!/usr/bin/env bash
set -euo pipefail

go  build \
    -o .build/getIE \
    -ldflags "-X main.BuildRev=$(git rev-parse --short HEAD)" \
    main.go
