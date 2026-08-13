#!/bin/bash

cd ui && npm ci && npm run build && cd ..

CGO_ENABLED=0 go build -ldflags="-s -w" -o goproxy ./cmd/goproxy