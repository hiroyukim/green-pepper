---
name: run-gp
description: Build and run the gp CLI (green-pepper) — CSV-driven HTTP request runner and its local web UI (gp serve). Use whenever you need to build gp, run a request via `gp run`, start/stop `gp serve` for manual verification, or exercise the web UI with curl.
---

# Running gp

`gp` is the CLI in this repo. It executes an HTTP request template once per
row of a CSV file. `gp serve` wraps the same logic in a local web UI.

## Build

```sh
go build -o gp .
```

Run this first any time source has changed. The binary is written to `./gp`.

## `gp run` (CLI, one-shot or CSV-driven)

```sh
gp run <request-file> [--data <csv-file>] [--env <env-file>] [--timeout <duration>]
```

- `--data`: CSV file, header row = variable names, each following row = one request.
  Omit it to run the template exactly once.
- `--env`: YAML file of default template variables (e.g. `base_url`). CSV values
  override env values when both define the same variable.
- `--timeout`: per-request timeout (default `30s`).
- Exit code is `0` iff every request returned 2xx; `1` otherwise.

Without CSV (single request, template vars must all resolve from `--env`
alone). `examples/request.yaml` references `{{id}}`, which `examples/env.yaml`
does not define, so running it without `--data` fails with `undefined
variable "id"` — that's expected; a template with no unresolved vars beyond
env would run fine:

```sh
./gp run examples/request.yaml --env examples/env.yaml
```

With CSV (repeats once per row of `examples/users.csv`, supplying `id` per row):

```sh
./gp run examples/request.yaml --data examples/users.csv --env examples/env.yaml
```

Expected output is a per-row status table followed by a `<passed>/<total> passed`
summary line.

## `gp serve` (local web UI)

```sh
gp serve <request-file> [--env <env-file>] [--port <port>] [--timeout <duration>]
```

The request template and env are fixed at startup; the browser only uploads a
CSV and triggers the run (`POST /run`, same execution path as `gp run --data`).

### Start it in the background

Pick a port that's likely free (check first if unsure: `lsof -i :8080` or just
try a less common port like `8099`), then background the process and capture
its PID so it can be stopped later:

```sh
./gp serve examples/request.yaml --env examples/env.yaml --port 8099 &
GP_SERVE_PID=$!
```

### Exercise it with curl

Fetch the index page:

```sh
curl -s http://localhost:8099/
```

Submit the CSV upload form (multipart, field name is `csv`):

```sh
curl -s -F "csv=@examples/users.csv" http://localhost:8099/run
```

This returns the rendered results page HTML (same table `gp run` prints, as
HTML instead of a terminal table).

### Stop it afterward

```sh
kill "$GP_SERVE_PID"
```

If the PID wasn't captured, find and kill it by port instead:

```sh
kill $(lsof -ti :8099)
```

## Notes / current limitations

- The web UI currently exposes only `GET /` and `POST /run` (CSV upload +
  run). There is no `/execute` endpoint and no `action=send|run|download`
  form yet — that richer request-builder UI is landing in a separate,
  not-yet-merged branch. Update this skill once that lands.
