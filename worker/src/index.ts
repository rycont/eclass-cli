import { Hono } from "hono";
import { bearerAuth } from "hono/bearer-auth";
import { StreamableHTTPTransport } from "@hono/mcp";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { runEclass, type Env } from "./eclass";
import toolsSpec from "./tools.json";

// 도구 정의의 원본은 Go 쪽(mcp.go)이다. build.sh가 `eclass mcp spec` 출력을
// src/tools.json으로 만들어 넣는다 — CLI(`eclass mcp`)와 Worker가 같은
// 도구 표를 쓰기 위해서다. tools.json은 생성물이라 커밋하지 않는다.
type Args = Record<string, string | string[] | undefined>;
type ToolSpec = {
  name: string;
  description: string;
  inputSchema: { type: string; properties: Record<string, unknown>; required: string[] };
  argv: string[];
};

// Go mcp.go의 renderArgv와 같은 규칙: "$name" 치환, "$name?" 선택, "$...name" 펼침.
function renderArgv(template: string[], a: Args): string[] {
  const out: string[] = [];
  for (const el of template) {
    if (el.startsWith("$...")) {
      const v = a[el.slice(4)];
      if (Array.isArray(v)) out.push(...v.map(String));
    } else if (el.startsWith("$")) {
      const optional = el.endsWith("?");
      const name = el.slice(1, optional ? -1 : undefined);
      const v = a[name];
      if (v === undefined || v === "") {
        if (optional) continue;
        throw new Error(`인자 ${name}가 필요합니다`);
      }
      out.push(String(v));
    } else {
      out.push(el);
    }
  }
  return out;
}

const TOOLS = (toolsSpec as { tools: ToolSpec[] }).tools;

function server(env: Env) {
  const s = new Server(
    { name: "eclass", version: "1.0.0" },
    { capabilities: { tools: {} } },
  );

  s.setRequestHandler(ListToolsRequestSchema, async () => ({
    tools: TOOLS.map((t) => ({
      name: t.name,
      description: t.description,
      inputSchema: t.inputSchema,
    })),
  }));

  s.setRequestHandler(CallToolRequestSchema, async (req) => {
    const tool = TOOLS.find((t) => t.name === req.params.name);
    if (!tool) throw new Error(`없는 도구: ${req.params.name}`);
    const argv = renderArgv(tool.argv, (req.params.arguments ?? {}) as Args);
    const { text, ok } = await runEclass(env, argv);
    return { content: [{ type: "text", text: text || "(출력 없음)" }], isError: !ok };
  });

  return s;
}

const app = new Hono<{ Bindings: Env }>();

// 클라이언트별로 토큰을 나눠 담는다: 기본 MCP_TOKEN + 용도별 MCP_TOKEN_* 시크릿.
app.use("/mcp", (c, next) =>
  bearerAuth({
    token: [c.env.MCP_TOKEN, c.env.MCP_TOKEN_DANIEL, c.env.MCP_TOKEN_RAKAZO].filter(
      (t): t is string => Boolean(t),
    ),
  })(c as never, next),
);

app.all("/mcp", async (c) => {
  const transport = new StreamableHTTPTransport();
  await server(c.env).connect(transport);
  return transport.handleRequest(c as never);
});

app.get("/", (c) => c.text("eclass MCP. POST /mcp (Authorization: Bearer <token>)\n"));

export default app;
