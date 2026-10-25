// Test-only module hooks: stub Cloudflare-only imports so the built worker
// bundle can load in plain Node. Never used in production code paths.
export async function resolve(specifier, context, next) {
  if (specifier === "cloudflare:sockets") return { url: "stub:sockets", shortCircuit: true };
  if (specifier.endsWith(".wasm")) return { url: "stub:wasm", shortCircuit: true };
  return next(specifier, context);
}
export async function load(url, context, next) {
  if (url === "stub:sockets") {
    return { format: "module", shortCircuit: true,
      source: 'export const connect=()=>{throw new Error("network blocked in test")};' };
  }
  if (url === "stub:wasm") return { format: "module", source: "export default undefined;", shortCircuit: true };
  return next(url, context);
}
