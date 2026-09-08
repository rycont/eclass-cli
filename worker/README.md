# eclass MCP on Cloudflare Workers

eclass-cli를 그대로 wasm으로 올린 원격 MCP 서버. 무료 플랜에서 돈다.

[![Deploy to Cloudflare](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/rycont/eclass-cli/tree/main/worker)

버튼이 KV와 시크릿을 물어본다. 직접 할 거면:

```bash
npm install
npx wrangler kv namespace create SESSION   # id를 wrangler.jsonc 에 넣는다
npx wrangler secret put ECLASS_ID          # 학번
npx wrangler secret put ECLASS_PASSWORD    # SAINT 비밀번호
npx wrangler secret put MCP_TOKEN          # openssl rand -hex 16
npm run deploy                             # wasm 빌드까지 같이 한다
```

```bash
claude mcp add --transport http eclass https://eclass-mcp.<계정>.workers.dev/mcp \
  --header "Authorization: Bearer $MCP_TOKEN"
```

## 도구

`course_ls`로 KJKEY를 얻어 나머지에 넣는다. 출력은 CLI와 같은 JSON.
디스크가 필요한 `init`·`sync`·`download`·`syllabus`와 대화형 `login`은 빠졌다.

도구를 늘리려면 `src/index.ts`의 `TOOLS`에 한 줄 추가한다.
플랫폼 차이는 Go 빌드 태그가 흡수한다 — `eclass/store_js.go`(KV+시크릿),
`eclass/transport_js.go`(raw 소켓, 왜 필요한지는 그 파일 주석에).

## 아는 문제

- 요청마다 Go 런타임을 새로 띄운다. 호출당 2~4초.
- 모듈 스코프에 Promise를 남겨 요청 간에 이어 붙이면 `Stream was cancelled`로 끊긴다.
