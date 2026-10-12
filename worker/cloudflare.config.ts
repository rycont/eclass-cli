import { bindings, defineConfig } from "cf/config";

/**
 * Secret-like files were detected but not read or migrated: .dev.vars.example. Only `secrets.required` entries are migrated.
 * @see https://developers.cloudflare.com/workers/configuration/secrets/
 */

export default defineConfig({
	worker: {
		name: "eclass-mcp",
		workersDev: true,
		compatibilityDate: "2026-09-01",
		compatibilityFlags: [
			"nodejs_compat",
		],
		entrypoint: "src/index.ts",
		env: {
			SESSION: bindings.kv({
				id: "79a6ec2a0de24e98b4bc22c058d84083",
			}),
		},
	},
});
