//go:build !js

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// eclass CLI를 MCP(Model Context Protocol) 서버로 노출한다.
//
//	stdio  로컬 클라이언트(Claude Code, Rakazo 등)가 프로세스로 띄워 붙는 방식
//	http   Streamable HTTP로 서빙하는 방식. Worker 배포는 worker/가 이 정의를
//	       그대로 가져가 서빙한다 (src/tools.json은 `eclass mcp spec`의 출력).
//
// 도구는 CLI 커맨드를 그대로 감싼다. 실행은 자기 자신을 서브프로세스로 띄워서
// 한다 — 커맨드가 fatal()로 exit해도 서버가 죽지 않고, SAINT ABAP 세션도
// 프로세스 종료로 확실히 닫힌다.

const mcpServerName = "eclass"
const mcpServerVersion = "1.0.0"

// protocolVersion은 클라이언트가 요청한 값을 그대로 돌려준다. 구현하는 메서드가
// initialize/ping/tools/* 로 버전 간에 같아서, 에코가 제일 호환이 잘 된다.
const mcpDefaultProtocol = "2025-06-18"

type mcpProperty struct {
	Type        string       `json:"type"`
	Description string       `json:"description"`
	Items       *mcpProperty `json:"items,omitempty"`
}

type mcpSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]mcpProperty `json:"properties"`
	Required   []string               `json:"required"`
}

// mcpTool 하나가 MCP 도구 하나. Argv는 CLI 인자 템플릿:
//
//	"$name"   인자 값으로 치환 (없으면 에러)
//	"$name?"  인자가 있을 때만 치환
//	"$...name" 배열 인자를 펼쳐서 붙임
//	그 외      그대로
type mcpTool struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Schema      mcpSchema `json:"inputSchema"`
	Argv        []string  `json:"argv"`
	// ReturnsFiles인 도구(download/syllabus)는 임시 디렉터리에서 실행하고
	// 만들어진 파일을 base64로 결과에 실어 준다 (worker의 write-shim과 같은 약속).
	ReturnsFiles bool `json:"returns_files,omitempty"`
}

func (t mcpTool) renderArgv(args map[string]any) ([]string, error) {
	var out []string
	for _, el := range t.Argv {
		switch {
		case strings.HasPrefix(el, "$..."):
			name := el[len("$..."):]
			v, ok := args[name]
			if !ok || v == nil {
				continue
			}
			arr, ok := v.([]any)
			if !ok {
				return nil, fmt.Errorf("인자 %s: 배열이 필요합니다", name)
			}
			for _, x := range arr {
				out = append(out, fmt.Sprint(x))
			}
		case strings.HasPrefix(el, "$"):
			optional := strings.HasSuffix(el, "?")
			name := strings.TrimSuffix(el[1:], "?")
			v, ok := args[name]
			s := fmt.Sprint(v)
			if !ok || v == nil || s == "" || s == "<nil>" {
				if optional {
					continue
				}
				return nil, fmt.Errorf("인자 %s가 필요합니다", name)
			}
			out = append(out, s)
		default:
			out = append(out, el)
		}
	}
	return out, nil
}

var (
	propKJKEY = mcpProperty{Type: "string", Description: "강좌 키. course_ls 로 확인한다."}
	propSEQ   = mcpProperty{Type: "string", Description: "목록 응답의 seq 값"}
)

func objSchema(props map[string]mcpProperty, required []string) mcpSchema {
	// MCP 클라이언트 스키마 검증(zod)은 properties/required의 null을 받지 않는다.
	// 없는 쪽은 빈 값으로 내보낸다.
	if props == nil {
		props = map[string]mcpProperty{}
	}
	if required == nil {
		required = []string{}
	}
	return mcpSchema{Type: "object", Properties: props, Required: required}
}

// 도구 표는 worker/src/index.ts 의 것과 같아야 한다 — 아니, 이제 이쪽이 원본이다.
// worker는 build.sh가 `eclass mcp spec` 출력을 src/tools.json으로 받아 같은 표를 쓴다.
var mcpTools = []mcpTool{
	{Name: "course_ls", Description: "수강 및 조교 담당 강좌 목록. 다른 도구에 넣을 KJKEY를 여기서 얻는다.",
		Schema: objSchema(nil, nil), Argv: []string{"course", "ls"}},
	{Name: "course_notices", Description: "강좌 공지사항 목록",
		Schema: objSchema(map[string]mcpProperty{"kjkey": propKJKEY}, []string{"kjkey"}),
		Argv:   []string{"course", "$kjkey", "notices"}},
	{Name: "course_notice", Description: "공지사항 본문과 첨부 목록",
		Schema: objSchema(map[string]mcpProperty{"kjkey": propKJKEY, "seq": propSEQ}, []string{"kjkey", "seq"}),
		Argv:   []string{"course", "$kjkey", "notice", "$seq"}},
	{Name: "course_files", Description: "강의자료 목록",
		Schema: objSchema(map[string]mcpProperty{"kjkey": propKJKEY}, []string{"kjkey"}),
		Argv:   []string{"course", "$kjkey", "files"}},
	{Name: "course_download", Description: "기존 CLI download 실행. 강좌 파일 번호로 받은 파일을 결과에 반환; 디스크 저장 없음.",
		Schema: objSchema(map[string]mcpProperty{"kjkey": propKJKEY, "file_seq": propSEQ}, []string{"kjkey", "file_seq"}),
		Argv:   []string{"course", "$kjkey", "download", "$file_seq"}, ReturnsFiles: true},
	{Name: "course_syllabus", Description: "기존 CLI syllabus 실행. 강의계획서 파일을 결과에 반환; 디스크 저장 없음.",
		Schema: objSchema(map[string]mcpProperty{"kjkey": propKJKEY}, []string{"kjkey"}),
		Argv:   []string{"course", "$kjkey", "syllabus"}, ReturnsFiles: true},
	{Name: "course_assignments", Description: "과제 목록",
		Schema: objSchema(map[string]mcpProperty{"kjkey": propKJKEY}, []string{"kjkey"}),
		Argv:   []string{"course", "$kjkey", "assignments"}},
	{Name: "course_assignment", Description: "과제 상세 (본문 + 첨부 + 제출 상태·제출 시각)",
		Schema: objSchema(map[string]mcpProperty{"kjkey": propKJKEY, "seq": propSEQ}, []string{"kjkey", "seq"}),
		Argv:   []string{"course", "$kjkey", "assignment", "$seq"}},
	{Name: "notifications", Description: "전체 강좌 알림",
		Schema: objSchema(nil, nil), Argv: []string{"notifications"}},
	{Name: "timetable", Description: "수강 강좌별 강의 시간",
		Schema: objSchema(nil, nil), Argv: []string{"timetable"}},
	{Name: "todo", Description: "미완료 할 일. kjkey 를 주면 그 강좌만.",
		Schema: objSchema(map[string]mcpProperty{"kjkey": {Type: "string", Description: propKJKEY.Description + " 생략하면 전체."}}, nil),
		Argv:   []string{"todo", "$kjkey?"}},
	{Name: "saint_menu", Description: "SAINT 화면 목록. 성적·장학금·학사 조회의 시작점.",
		Schema: objSchema(nil, nil), Argv: []string{"saint", "menu"}},
	{Name: "saint_open", Description: "SAINT 화면 열기. 화면·동작 이름은 부분 일치로 찾는다.",
		Schema: objSchema(map[string]mcpProperty{
			"screen": {Type: "string", Description: `화면 이름. 예: "학기별성적", "장학금 수혜내역"`},
			"args":   {Type: "array", Items: &mcpProperty{Type: "string"}, Description: `동작 이름 또는 입력칸=값. 예: ["전체성적"], ["소속구분=대학", "검색"]`},
		}, []string{"screen"}),
		Argv: []string{"saint", "open", "$screen", "$...args"}},
}

func findTool(name string) *mcpTool {
	for i := range mcpTools {
		if mcpTools[i].Name == name {
			return &mcpTools[i]
		}
	}
	return nil
}

// --- JSON-RPC ---

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// executeFunc은 도구 호출 하나를 실행해 텍스트 결과와 에러 여부를 돌려준다.
// 테스트에서 가짜 실행기를 끼울 수 있게 엔진 필드로 둔다.
type executeFunc func(t *mcpTool, args map[string]any) (text string, isError bool)

type mcpEngine struct {
	execute executeFunc
}

func newMCPEngine() *mcpEngine { return &mcpEngine{execute: executeTool} }

// handle은 JSON-RPC 메시지 하나를 받아 응답을 돌려준다.
// notification(응답하면 안 되는 것)은 nil을 돌려준다.
func (e *mcpEngine) handle(raw []byte) []byte {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return marshal(rpcResponse{JSONRPC: "2.0", ID: nil, Error: &rpcError{Code: -32700, Message: "파싱 실패: " + err.Error()}})
	}
	if req.Method == "" {
		// method 없는 메시지(클라이언트의 response)는 무시한다.
		return nil
	}

	respond := func(result any) []byte {
		return marshal(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
	}
	fail := func(code int, msg string) []byte {
		return marshal(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: msg}})
	}

	switch req.Method {
	case "initialize":
		version := mcpDefaultProtocol
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(req.Params, &p) == nil && p.ProtocolVersion != "" {
			version = p.ProtocolVersion
		}
		return respond(map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": mcpServerName, "version": mcpServerVersion},
		})
	case "ping":
		return respond(map[string]any{})
	case "notifications/initialized", "notifications/cancelled", "notifications/progress":
		return nil
	case "tools/list":
		return respond(map[string]any{"tools": mcpTools})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return fail(-32602, "tools/call 인자 파싱 실패: "+err.Error())
		}
		t := findTool(p.Name)
		if t == nil {
			return fail(-32602, "없는 도구: "+p.Name)
		}
		text, isError := e.execute(t, p.Arguments)
		if text == "" {
			text = "(출력 없음)"
		}
		return respond(map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"isError": isError,
		})
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		return fail(-32601, "알 수 없는 메서드: "+req.Method)
	}
}

func marshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// --- 도구 실행: 자기 자신을 서브프로세스로 ---

// 한 번에 하나씩만 실행한다. 세션 파일 쓰기 경합과 SAINT 사용자당 세션 한도 때문.
var mcpExecMu sync.Mutex

const mcpExecTimeout = 3 * time.Minute

// 파일 하나의 MCP 응답 상한. 강의 슬라이드 PDF가 수백 KB라 64MB면 넉넉하다.
const mcpFileCap = 64 << 20

func executeTool(t *mcpTool, args map[string]any) (string, bool) {
	argv, err := t.renderArgv(args)
	if err != nil {
		return marshalText(map[string]any{"error": err.Error()}), true
	}
	exe, err := os.Executable()
	if err != nil {
		return marshalText(map[string]any{"error": "실행 파일 경로 확인 실패: " + err.Error()}), true
	}

	ctx, cancel := context.WithTimeout(context.Background(), mcpExecTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, argv...)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	var dir string
	if t.ReturnsFiles {
		dir, err = os.MkdirTemp("", "eclass-mcp-")
		if err != nil {
			return marshalText(map[string]any{"error": "임시 디렉터리 생성 실패: " + err.Error()}), true
		}
		defer os.RemoveAll(dir)
		cmd.Dir = dir
	}

	mcpExecMu.Lock()
	runErr := cmd.Run()
	mcpExecMu.Unlock()

	outText := strings.TrimSpace(stdout.String())
	errText := strings.TrimSpace(stderr.String())
	if ctx.Err() == context.DeadlineExceeded {
		return marshalText(map[string]any{"error": "실행 시간 초과 (" + mcpExecTimeout.String() + ")", "stdout": outText}), true
	}
	if runErr != nil {
		if errText != "" {
			return errText, true
		}
		return marshalText(map[string]any{"error": runErr.Error(), "stdout": outText}), true
	}
	if !t.ReturnsFiles {
		return outText, false
	}

	// download/syllabus: 임시 디렉터리에 생긴 파일을 싣는다.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return marshalText(map[string]any{"error": "결과 디렉터리 읽기 실패: " + err.Error()}), true
	}
	type exportedFile struct {
		Path     string `json:"path"`
		ByteSize int64  `json:"byte_size"`
		SHA256   string `json:"sha256"`
		Base64   string `json:"base64"`
	}
	var files []exportedFile
	for _, ent := range entries {
		if !ent.Type().IsRegular() {
			continue
		}
		data, err := os.ReadFile(dir + string(os.PathSeparator) + ent.Name())
		if err != nil {
			return marshalText(map[string]any{"error": "결과 파일 읽기 실패: " + err.Error()}), true
		}
		if len(data) > mcpFileCap {
			return marshalText(map[string]any{"error": fmt.Sprintf("파일이 너무 큼 (%s, %d bytes)", ent.Name(), len(data))}), true
		}
		sum := sha256.Sum256(data)
		files = append(files, exportedFile{
			Path:     ent.Name(),
			ByteSize: int64(len(data)),
			SHA256:   hex.EncodeToString(sum[:]),
			Base64:   base64.StdEncoding.EncodeToString(data),
		})
	}
	if len(files) == 0 {
		return marshalText(map[string]any{"ok": false, "error": "CLI가 파일을 만들지 않음", "result": json.RawMessage(stdout.Bytes())}), true
	}
	return marshalText(map[string]any{"ok": true, "result": json.RawMessage(stdout.Bytes()), "files": files}), false
}

func marshalText(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSpace(buf.String())
}

// --- stdio ---

func cmdMCPStdio() {
	engine := newMCPEngine()
	// 줄 단위 JSON-RPC. 다운로드 결과가 수 MB까지 갈 수 있어 고정 버퍼 스캐너는 안 쓴다.
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	outW := bufio.NewWriter(os.Stdout)
	enc := json.NewEncoder(outW)
	enc.SetEscapeHTML(false)
	for {
		line, err := in.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			if resp := engine.handle(trimmed); resp != nil {
				_ = enc.Encode(json.RawMessage(resp))
				_ = outW.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

// --- Streamable HTTP ---

// 인증 토큰은 --token 또는 ECLASS_MCP_TOKEN. 루프백이 아니면 토큰을 강제한다:
// 이 엔드포인트는 학사 계정 읽기 권한 그 자체라 인증 없이 열리면 안 된다.
func cmdMCPHTTP(addr, token string) {
	if token == "" {
		token = os.Getenv("ECLASS_MCP_TOKEN")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		fatal(fmt.Errorf("주소 형식이 이상합니다: %s (%v)", addr, err))
	}
	loopback := host == "" || host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
	if token == "" && !loopback {
		fatal(fmt.Errorf("루프백이 아닌 주소(%s)에 열 때는 --token 또는 ECLASS_MCP_TOKEN이 필요합니다", addr))
	}

	engine := newMCPEngine()
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodPost:
			body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
			if err != nil || len(bytes.TrimSpace(body)) == 0 {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			resp := engine.handle(bytes.TrimSpace(body))
			if resp == nil {
				// notification만 온 경우
				w.WriteHeader(http.StatusAccepted)
				return
			}
			// initialize에는 세션 id를 실어 준다 (stateless라 검증은 하지 않는다).
			var probe rpcRequest
			if json.Unmarshal(body, &probe) == nil && probe.Method == "initialize" {
				sid := make([]byte, 16)
				_, _ = rand.Read(sid)
				w.Header().Set("Mcp-Session-Id", hex.EncodeToString(sid))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(resp)
		case http.MethodGet:
			// 서버가 여는 SSE 스트림은 지원하지 않는다 (스펙상 선택 사항).
			w.Header().Set("Allow", "POST, DELETE")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "eclass MCP. POST /mcp (Authorization: Bearer <token>)\n")
	})

	fmt.Fprintf(os.Stderr, "eclass MCP (Streamable HTTP) listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fatal(err)
	}
}

// --- 엔트리 ---

func cmdMCP(args []string) {
	mode := "stdio"
	rest := args
	if len(args) > 0 {
		switch args[0] {
		case "stdio", "http", "spec":
			mode = args[0]
			rest = args[1:]
		}
	}

	switch mode {
	case "stdio":
		cmdMCPStdio()
	case "spec":
		// worker/build.sh가 이 출력을 src/tools.json으로 받아 같은 도구 표를 쓴다.
		out(map[string]any{"tools": mcpTools})
	case "http":
		addr := "127.0.0.1:8080"
		token := ""
		for i := 0; i < len(rest); i++ {
			a := rest[i]
			name, val, hasVal := strings.Cut(a, "=")
			if !hasVal && i+1 < len(rest) {
				name, val, hasVal = a, rest[i+1], true
				i++
			}
			switch name {
			case "--addr":
				addr = val
			case "--token":
				token = val
			default:
				usageError("mcp", "알 수 없는 옵션: "+a)
			}
		}
		cmdMCPHTTP(addr, token)
	}
}
