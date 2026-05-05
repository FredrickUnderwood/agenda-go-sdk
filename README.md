# agenda-go-sdk

Bare-bones observability SDK for services deployed by [agenda](https://github.com/agenda/agenda).

The SDK is intentionally a **thin client**: it carries only the runtime pieces
a deployed service needs to plug into agenda's observability story (logs now,
metrics and traces next). It does **not** depend on any agenda server-side
code — your `go.mod` only picks up zap.

## Status

| Package        | Status         | Notes                                          |
| -------------- | -------------- | ---------------------------------------------- |
| `log/`         | Stable (v0.x)  | zap-based, async daily-rotating file writer    |
| `metric/`      | Reserved       | Placeholder; OpenTelemetry-based, see roadmap  |
| `trace/`       | Reserved       | Placeholder; OpenTelemetry-based, see roadmap  |

## Install

```bash
go get github.com/agenda-go-sdk
```

## Quick start (log)

```go
package main

import (
    "github.com/agenda-go-sdk/log"
    "go.uber.org/zap"
)

func main() {
    _ = log.Init(log.Config{AppName: "checkout"})
    defer log.Shutdown()

    log.L().Info("server started", zap.Int("port", 8080))
}
```

That's it. Logs land in `./logs/checkout-2026-05-04.log` and stream to stdout
in parallel.

## How the log file path is derived

```
<LogDir>/<AppName>-YYYY-MM-DD.log
```

`LogDir` defaults to `./logs`, but for agenda-deployed docker-compose services
the value comes from `AGENDA_LOG_DIR=/var/log/agenda`, which agenda injects
into every container together with a bind mount:

```
host: <workspace_root>/<host>/<repo_path>/<branch>/logs   ⇄   container: /var/log/agenda
```

So a deployed app writes to `/var/log/agenda/<app>-YYYY-MM-DD.log` inside the
container, and the same file appears on the host at:

```
<workspace_root>/<host>/<repo_path>/<branch>/logs/<app>-YYYY-MM-DD.log
```

You can `cat` / `tail -f` it directly on the host — `docker logs` and
`docker exec` are not required. Different branches of the same app keep
their logs cleanly separated because the bind mount root is per-branch.

### What agenda injects into each compose service

When `compose up` runs, agenda generates a managed override at
`<LocalPath>/.agenda/compose.override.yml` and merges it on top of the
user's compose file via `-f`. For each target service it adds:

```yaml
volumes:
  - <LocalPath>/logs:/var/log/agenda
environment:
  - AGENDA_APP_NAME=<app.Name>
  - AGENDA_LOG_DIR=/var/log/agenda
  - AGENDA_REPO_BRANCH=<repo.Branch>
```

This means deployed services get correct log paths with **no compose-file
changes on the user side** — the SDK reads the env vars and writes to the
mount.

## Configuration

Field precedence is (highest first):

1. Explicit value on the `Config` struct passed to `Init`
2. Matching `AGENDA_*` environment variable (set by agenda at deploy time)
3. Built-in default

| Config field   | Env var             | Default     | Notes                                       |
| -------------- | ------------------- | ----------- | ------------------------------------------- |
| `AppName`      | `AGENDA_APP_NAME`   | `app`       | Filename prefix.                             |
| `LogDir`       | `AGENDA_LOG_DIR`    | `./logs`    | Tilde-prefix paths (`~/...`) are expanded.   |
| `Level`        | `AGENDA_LOG_LEVEL`  | `info`      | One of `debug \| info \| warn \| error`.    |
| `Console`      | —                   | `true`      | Mirrors output to stdout.                   |
| `BufSize`      | —                   | `4096`      | Async writer channel capacity.              |
| `DisableFile`  | —                   | `false`     | Set true to skip the file sink.             |

## Why the writer drops on full

The async writer keeps the calling goroutine free of disk-IO latency. When
the channel is full (typically because the disk has stalled), writes are
**dropped** rather than blocked, so a slow disk never turns into a stalled
request. If you cannot tolerate drops, raise `BufSize` or set
`DisableFile: true` and rely on stdout collection.

## Relationship with agenda

```
+----------------------+         require        +-----------------+
|  agenda server       |  -- (planned) ------>  | agenda-go-sdk   |
|  (deployer / poller) |                        +-------^---------+
+----------------------+                                |
        | injects ENV at deploy time                    | import
        |   AGENDA_APP_NAME / AGENDA_LOG_DIR / ...      |
        v                                               |
+--------------------+   compose / run    +--------------------+
| business service A | <----------------- |  business service  |
| business service B |                    |     code           |
+--------------------+                    +--------------------+
```

- The SDK is **single-direction**: agenda may depend on it; this SDK never
  imports agenda.
- This SDK is the supported way for any service to participate in agenda's
  observability flow. Direct use of zap is fine for local one-offs but loses
  the env-var convention agenda relies on.

## Roadmap

- `metric/` — OTel MeterProvider with RED-style helpers and OTLP HTTP
  exporter; `AGENDA_METRIC_ENDPOINT` env knob.
- `trace/` — OTel TracerProvider plus a gin / net/http middleware. Once
  shipped, the existing `log.Info(ctx, msg, ...)` helpers will automatically
  attach `trace_id` / `span_id` — no caller-side change required.
- agenda main repo: replace `internal/logger` with `agenda-go-sdk/log`.

## Development

```bash
go build ./...
go test ./... -race
```
