import { connect } from "cloudflare:sockets";
import "./wasm_exec.js";
import wasm from "./eclass.wasm";

type KV = {
  get(key: string): Promise<string | null>;
  put(key: string, value: string): Promise<void>;
  delete(key: string): Promise<void>;
};

export type Env = {
  SESSION: KV;
  ECLASS_ID: string;
  ECLASS_PASSWORD: string;
  MCP_TOKEN: string;
};

const toBytes = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));

function toB64(u8: Uint8Array): string {
  let s = "";
  for (let i = 0; i < u8.length; i += 0x8000) {
    s += String.fromCharCode(...u8.subarray(i, i + 0x8000));
  }
  return btoa(s);
}

// Workers의 fetch()는 서강대의 불완전한 인증서 체인을 거부한다(526). raw 소켓은
// 그 검증 경로를 안 타므로 여기로 우회한다. 자세한 사정은 eclass/transport_js.go 참고.
async function socketFetch(hostname: string, port: string, b64req: string): Promise<string> {
  const sock = connect({ hostname, port: Number(port) }, { secureTransport: "on" });
  try {
    const w = sock.writable.getWriter();
    await w.write(toBytes(b64req));
    w.releaseLock();

    const reader = sock.readable.getReader();
    const chunks: Uint8Array[] = [];
    let total = 0;
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      chunks.push(value);
      total += value.length;
    }
    reader.releaseLock();

    const out = new Uint8Array(total);
    let off = 0;
    for (const c of chunks) {
      out.set(c, off);
      off += c.length;
    }
    return toB64(out);
  } finally {
    // 이걸 빠뜨리면 다음 요청이 "Stream was cancelled"로 깨진다.
    try {
      await sock.close();
    } catch {
      /* 이미 닫힘 */
    }
  }
}

/**
 * CLI 한 번 실행. 출력은 커맨드가 찍은 JSON 한 줄이다.
 *
 * 여기서 모듈 스코프에 Promise를 남겨 요청 간에 이어 붙이면 안 된다. Workers가
 * 이전 요청 컨텍스트의 I/O로 간주해 "Stream was cancelled"로 끊는다(직렬화 큐를
 * 넣었다가 실제로 깨졌다). 상태는 전부 이 함수 호출 안에서 시작하고 끝난다.
 *
 * ponytail: 콘솔 가로채기와 호스트 훅은 전역이라 한 아이솔레이트에서 동시 실행이
 * 겹치면 출력이 섞인다. 에이전트 호출은 순차라 실측상 문제 없었다. 겹치기 시작하면
 * 그때 Durable Object로 요청을 격리할 것.
 */
export async function runEclass(
  env: Env,
  args: string[],
): Promise<{ text: string; ok: boolean }> {
  {
    const g = globalThis as Record<string, unknown>;

    // Go 쪽 transport_js.go / store_js.go 가 이 이름들로 호스트를 부른다.
    g.eclassSocketFetch = socketFetch;
    g.eclassStoreGet = async (name: string) =>
      name === "credentials"
        ? JSON.stringify({ id: env.ECLASS_ID, password: env.ECLASS_PASSWORD })
        : await env.SESSION.get(name);
    g.eclassStoreSet = async (name: string, value: string | null) =>
      value === null ? env.SESSION.delete(name) : env.SESSION.put(name, value);

    // wasm_exec.js의 fs 셔틀은 fd를 구분하지 않고 전부 console로 보낸다.
    // 성공이든 실패든 JSON 한 줄이라 합쳐 받고 종료 코드로 성패를 가른다.
    const lines: string[] = [];
    const orig = { log: console.log, warn: console.warn, error: console.error };
    const sink = (...a: unknown[]) => void lines.push(a.join(" "));
    console.log = console.warn = console.error = sink;

    let code = 0;
    try {
      const go = new (g.Go as new () => any)();
      go.argv = ["eclass", ...args];
      go.exit = (c: number) => {
        code = c;
      };
      await go.run(await WebAssembly.instantiate(wasm, go.importObject));
    } finally {
      Object.assign(console, orig);
    }

    return { text: lines.join("\n").trim(), ok: code === 0 };
  }
}
