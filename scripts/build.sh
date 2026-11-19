#!/usr/bin/env bash
set -e
cd "$(dirname "$0")/.."

echo "================================================================"
echo "  BUILDING WIREDROP (100% GO STANDARD LIBRARY - MULTI-PLATFORM)"
echo "================================================================"
echo ""

mkdir -p dist

export CGO_ENABLED=0

echo "[*] 1/3 Compiling Linux AMD64 (dist/wiredrop-linux-amd64) ..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/wiredrop-linux-amd64 .

echo "[*] 2/3 Compiling Linux ARM64 (dist/wiredrop-linux-arm64) ..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/wiredrop-linux-arm64 .

echo "[*] 3/3 Compiling Windows AMD64 (dist/wiredrop.exe) ..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/wiredrop.exe .

echo ""
echo "================================================================"
echo "  [OK] ALL BUILDS COMPLETED SUCCESSFULLY!"
echo "================================================================"
ls -lh dist/wiredrop*
