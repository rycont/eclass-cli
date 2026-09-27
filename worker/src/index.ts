import { Hono } from "hono";
import { bearerAuth } from "hono/bearer-auth";
import { StreamableHTTPTransport } from "@hono/mcp";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { runEclass, type Env } from "./eclass";

type Args = Record<string, string | string[] | undefined>;
type Tool = {
  name: string;
  description: string;
  properties?: Record<string, unknown>;
  required?: string[];
  argv: (a: Args) => string[];
};

const KJKEY = { type: "string", description: "강좌 키. course_ls 로 확인한다." };
const SEQ = { type: "string", description: "목록 응답의 seq 값" };

// 파일시스템을 쓰는 커맨드(init/sync/download/syllabus)와 대화형 login은 뺐다.
// Workers에는 디스크가 없고, 자격증명은 시크릿에서 온다.
const TOOLS: Tool[] = [
  { name: "course_ls", description: "수강 강좌 목록. 다른 도구에 넣을 KJKEY를 여기서 얻는다.", argv: () => ["course", "ls"] },
  { name: "course_notices", description: "강좌 공지사항 목록", properties: { kjkey: KJKEY }, required: ["kjkey"], argv: (a) => ["course", String(a.kjkey), "notices"] },
  { name: "course_notice", description: "공지사항 본문과 첨부 목록", properties: { kjkey: KJKEY, seq: SEQ }, required: ["kjkey", "seq"], argv: (a) => ["course", String(a.kjkey), "notice", String(a.seq)] },
  { name: "course_files", description: "강의자료 목록", properties: { kjkey: KJKEY }, required: ["kjkey"], argv: (a) => ["course", String(a.kjkey), "files"] },
  { name: "course_assignments", description: "과제 목록", properties: { kjkey: KJKEY }, required: ["kjkey"], argv: (a) => ["course", String(a.kjkey), "assignments"] },
  { name: "course_assignment", description: "과제 상세 (본문 + 첨부 + 제출 상태·제출 시각)", properties: { kjkey: KJKEY, seq: SEQ }, required: ["kjkey", "seq"], argv: (a) => ["course", String(a.kjkey), "assignment", String(a.seq)] },
  { name: "notifications", description: "전체 강좌 알림", argv: () => ["notifications"] },
  { name: "timetable", description: "수강 강좌별 강의 시간", argv: () => ["timetable"] },
  { name: "todo", description: "미완료 할 일. kjkey 를 주면 그 강좌만.", properties: { kjkey: { ...KJKEY, description: KJKEY.description + " 생략하면 전체." } }, argv: (a) => (a.kjkey ? ["todo", String(a.kjkey)] : ["todo"]) },
  { name: "saint_menu", description: "SAINT 화면 목록. 성적·장학금·학사 조회의 시작점.", argv: () => ["saint", "menu"] },
  {
    name: "saint_open",
    description: "SAINT 화면 열기. 화면·동작 이름은 부분 일치로 찾는다.",
    properties: {
      screen: { type: "string", description: '화면 이름. 예: "학기별성적", "장학금 수혜내역"' },
      args: { type: "array", items: { type: "string" }, description: '동작 이름 또는 입력칸=값. 예: ["전체성적"], ["소속구분=대학", "검색"]' },
    },
    required: ["screen"],
    argv: (a) => ["saint", "open", String(a.screen), ...((a.args as string[]) ?? [])],
  },
];

function server(env: Env) {
  const s = new Server(
    { name: "eclass", version: "1.0.0" },
    { capabilities: { tools: {} } },
  );

  s.setRequestHandler(ListToolsRequestSchema, async () => ({
    tools: TOOLS.map((t) => ({
      name: t.name,
      description: t.description,
      inputSchema: { type: "object", properties: t.properties ?? {}, required: t.required ?? [] },
    })),
  }));

  s.setRequestHandler(CallToolRequestSchema, async (req) => {
    const tool = TOOLS.find((t) => t.name === req.params.name);
    if (!tool) throw new Error(`없는 도구: ${req.params.name}`);
    const { text, ok } = await runEclass(env, tool.argv(req.params.arguments ?? {}));
    return { content: [{ type: "text", text: text || "(출력 없음)" }], isError: !ok };
  });

  return s;
}

const app = new Hono<{ Bindings: Env }>();

app.use("/mcp", (c, next) => bearerAuth({ token: c.env.MCP_TOKEN })(c, next));

app.all("/mcp", async (c) => {
  const transport = new StreamableHTTPTransport();
  await server(c.env).connect(transport);
  return transport.handleRequest(c);
});

app.get("/", (c) => c.text("eclass MCP. POST /mcp (Authorization: Bearer <MCP_TOKEN>)\n"));

export default app;
