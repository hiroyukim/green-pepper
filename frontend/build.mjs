// Bundles the gp serve frontend into ../internal/server/static/dist, which
// internal/server/server.go embeds via go:embed and serves at /static/.
// Run `npm run build` here before `go build`/`go test` at the repo root
// whenever anything under src/ changes.
import * as esbuild from 'esbuild';

await esbuild.build({
  entryPoints: [
    'src/main.css',
    'src/pages/index.js',
    'src/pages/progress.js',
    'src/pages/results.js',
  ],
  outdir: '../internal/server/static/dist',
  entryNames: '[name]',
  assetNames: 'assets/[name]-[hash]',
  bundle: true,
  minify: true,
  format: 'esm',
  target: 'es2020',
  loader: { '.woff2': 'file' },
  logLevel: 'info',
});
