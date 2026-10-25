// Regression test for the 2026-10-25 incident: the /mcp Hono handler passed
// the request context instead of c.env into server(), so the wasm host bridge
// saw undefined credentials/SESSION and eClass rejected empty logins.
// Drives the BUILT bundle through the real Hono handler with dummy bindings
// and asserts the bindings reach the host bridge. No network, no real secrets.
import assert from "node:assert/strict";
import path from "node:path";

const bundle = path.resolve(import.meta.dirname, "../.cloudflare/output/v0/workers/default/bundle/index.js");
const { default: app } = await import(bundle);

const sessionGets = [];
const dummyEnv = {
  ECLASS_ID: "dummy-id",
  ECLASS_PASSWORD: "dummy-pw",
  MCP_TOKEN: "test-token",
  SESSION: {
    get: async (k) => { sessionGets.push(k); return null; },
    put: async () => {},
    delete: async () => {},
  },
};

const rpc = (body, sid) => app.request("http://test.local/mcp", {
  method: "POST",
  headers: {
    authorization: "Bearer test-token",
    "content-type": "application/json",
    accept: "application/json, text/event-stream",
    ...(sid ? { "mcp-session-id": sid } : {}),
  },
  body: JSON.stringify(body),
}, dummyEnv);

const init = await rpc({ jsonrpc: "2.0", id: 0, method: "initialize",
  params: { protocolVersion: "2025-03-26", capabilities: {}, clientInfo: { name: "env-boundary-test", version: "0" } } });
assert.equal(init.status, 200, "initialize must succeed");
const sid = init.headers.get("mcp-session-id") || undefined;
await rpc({ jsonrpc: "2.0", method: "notifications/initialized" }, sid);

const callP = rpc({ jsonrpc: "2.0", id: 1, method: "tools/call",
  params: { name: "course_ls", arguments: {} } }, sid);

// Wait until runEclass installs the host bridges, then inspect them.
const deadline = Date.now() + 5000;
while (!globalThis.eclassStoreGet) {
  if (Date.now() > deadline) throw new Error("tool call never reached runEclass");
  await new Promise((r) => setImmediate(r));
}
const creds = JSON.parse(await globalThis.eclassStoreGet("credentials"));
assert.equal(creds.id, "dummy-id", "ECLASS_ID must reach the wasm host bridge");
assert.equal(creds.password, "dummy-pw", "ECLASS_PASSWORD must reach the wasm host bridge");
await globalThis.eclassStoreGet("session");
assert.deepEqual(sessionGets, ["session"], "session load must hit the bound KV namespace");

const res = await callP; // wasm is stubbed; the call fails internally. Not under test.
await res.text().catch(() => {});
console.log("env boundary ok: bindings reach the wasm host bridge through the Hono handler");
