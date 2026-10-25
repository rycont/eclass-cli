package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func callEngine(e *mcpEngine, body string) map[string]any {
	resp := e.handle([]byte(body))
	if resp == nil {
		return nil
	}
	var v map[string]any
	if err := json.Unmarshal(resp, &v); err != nil {
		panic(err)
	}
	return v
}

func TestMCPInitializeEchoesProtocolVersion(t *testing.T) {
	e := &mcpEngine{execute: func(t *mcpTool, a map[string]any) (string, bool) { return "[]", false }}
	res := callEngine(e, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`)
	result := res["result"].(map[string]any)
	if result["protocolVersion"] != "2025-03-26" {
		t.Fatalf("클라이언트 버전을 에코해야 함: %v", result["protocolVersion"])
	}
	if result["serverInfo"].(map[string]any)["name"] != "eclass" {
		t.Fatalf("serverInfo.name: %v", result["serverInfo"])
	}
}

func TestMCPInitializeDefaultVersion(t *testing.T) {
	e := &mcpEngine{execute: func(t *mcpTool, a map[string]any) (string, bool) { return "[]", false }}
	res := callEngine(e, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if res["result"].(map[string]any)["protocolVersion"] != mcpDefaultProtocol {
		t.Fatalf("기본 버전: %v", res)
	}
}

func TestMCPNotificationGetsNoResponse(t *testing.T) {
	e := &mcpEngine{execute: func(t *mcpTool, a map[string]any) (string, bool) { return "[]", false }}
	if resp := e.handle([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); resp != nil {
		t.Fatalf("notification에 응답하면 안 됨: %s", resp)
	}
}

func TestMCPToolsListMatchesSpec(t *testing.T) {
	e := &mcpEngine{execute: func(t *mcpTool, a map[string]any) (string, bool) { return "[]", false }}
	res := callEngine(e, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := res["result"].(map[string]any)["tools"].([]any)
	if len(tools) != len(mcpTools) {
		t.Fatalf("도구 수: %d != %d", len(tools), len(mcpTools))
	}
	first := tools[0].(map[string]any)
	if first["name"] != "course_ls" {
		t.Fatalf("첫 도구: %v", first["name"])
	}
	if first["inputSchema"].(map[string]any)["type"] != "object" {
		t.Fatalf("inputSchema.type: %v", first["inputSchema"])
	}
}

func TestMCPToolsCallRunsTool(t *testing.T) {
	var gotTool string
	var gotArgs map[string]any
	e := &mcpEngine{execute: func(tool *mcpTool, a map[string]any) (string, bool) {
		gotTool, gotArgs = tool.Name, a
		return `[{"kjkey":"A1"}]`, false
	}}
	res := callEngine(e, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"course_ls","arguments":{}}}`)
	result := res["result"].(map[string]any)
	if result["isError"] != false {
		t.Fatalf("isError: %v", result["isError"])
	}
	content := result["content"].([]any)[0].(map[string]any)
	if !strings.Contains(content["text"].(string), "kjkey") {
		t.Fatalf("text: %v", content["text"])
	}
	if gotTool != "course_ls" || gotArgs == nil {
		t.Fatalf("실행 인자: %s %v", gotTool, gotArgs)
	}
}

func TestMCPToolsCallUnknownTool(t *testing.T) {
	e := &mcpEngine{execute: func(tool *mcpTool, a map[string]any) (string, bool) { return "", false }}
	res := callEngine(e, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nope"}}`)
	if res["error"].(map[string]any)["code"].(float64) != -32602 {
		t.Fatalf("없는 도구는 -32602: %v", res["error"])
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	e := &mcpEngine{execute: func(tool *mcpTool, a map[string]any) (string, bool) { return "", false }}
	res := callEngine(e, `{"jsonrpc":"2.0","id":5,"method":"resources/list"}`)
	if res["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Fatalf("모르는 메서드는 -32601: %v", res["error"])
	}
}

func TestRenderArgv(t *testing.T) {
	todo := findTool("todo")
	argv, err := todo.renderArgv(map[string]any{})
	if err != nil || len(argv) != 1 || argv[0] != "todo" {
		t.Fatalf("todo 인자 생략: %v %v", argv, err)
	}
	argv, err = todo.renderArgv(map[string]any{"kjkey": "A1"})
	if err != nil || len(argv) != 2 || argv[1] != "A1" {
		t.Fatalf("todo kjkey: %v %v", argv, err)
	}

	notice := findTool("course_notice")
	if _, err = notice.renderArgv(map[string]any{"kjkey": "A1"}); err == nil {
		t.Fatal("필수 인자 빠지면 에러여야 함")
	}
	argv, err = notice.renderArgv(map[string]any{"kjkey": "A1", "seq": "3"})
	if err != nil || strings.Join(argv, " ") != "course A1 notice 3" {
		t.Fatalf("notice: %v %v", argv, err)
	}

	open := findTool("saint_open")
	argv, err = open.renderArgv(map[string]any{"screen": "학기별성적", "args": []any{"전체성적", "검색"}})
	if err != nil || strings.Join(argv, " ") != "saint open 학기별성적 전체성적 검색" {
		t.Fatalf("saint_open spread: %v %v", argv, err)
	}
	argv, err = open.renderArgv(map[string]any{"screen": "학기별성적"})
	if err != nil || strings.Join(argv, " ") != "saint open 학기별성적" {
		t.Fatalf("saint_open 인자 생략: %v %v", argv, err)
	}
}
