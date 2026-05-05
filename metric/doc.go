// Package metric is reserved for the agenda SDK's metrics surface.
//
// It is intentionally empty in this release. The planned implementation will
// wrap an OpenTelemetry MeterProvider with the same opinionated bootstrap
// style as the log package, so that a service can call:
//
//	_ = metric.Init(metric.Config{AppName: "checkout"})
//	defer metric.Shutdown()
//
// and immediately export RED-style request metrics plus a small number of
// runtime gauges (goroutines, GC pause) to whatever backend agenda is
// configured to scrape.
//
// TODO:
//   - Pick a default exporter (OTLP HTTP is the current candidate).
//   - Decide whether to expose the raw *metric.Meter or only sugared
//     helpers (Counter / Histogram / Gauge).
//   - Wire AGENDA_METRIC_ENDPOINT / AGENDA_METRIC_INTERVAL env vars.
package metric
