#!/usr/bin/env bash
# wasm 바이너리와 Go가 요구하는 글루 스크립트를 src/ 에 만든다. 둘 다 생성물이라 커밋하지 않는다.
set -euo pipefail
cd "$(dirname "$0")"
GOOS=js GOARCH=wasm go build -o src/eclass.wasm ..
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" src/wasm_exec.js
ls -la src/eclass.wasm
