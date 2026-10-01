Run from worker/:

```
npx esbuild src/write-shim.ts --bundle --platform=node --format=esm --outfile=test/write-shim.mjs
node test/write-shim.test.mjs
rm test/write-shim.mjs
```

Tests cover read/write/stat, separate stdout, path confinement, file size cap,
and binary byte preservation. Generated bundles must not be committed.
