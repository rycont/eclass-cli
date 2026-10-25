//go:build js && wasm

package main

import (
	"fmt"
	"os"
)

// wasm(Cloudflare Workers)에서는 MCP 서버를 띄울 수 없다 — 소켓 리슨이 안 된다.
// Worker에서의 MCP 서빙은 worker/의 TypeScript 층이 맡고, 도구 정의는
// `eclass mcp spec`(네이티브 빌드)의 출력을 공유한다.
func cmdMCP(args []string) {
	fmt.Fprintln(os.Stderr, `{"error":"mcp 서버는 wasm 빌드에서 쓸 수 없습니다 (worker가 HTTP로 서빙합니다)"}`)
	os.Exit(1)
}
