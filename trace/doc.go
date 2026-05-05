// Package trace is reserved for the agenda SDK's distributed tracing surface.
//
// It is intentionally empty in this release. The planned implementation will
// wrap an OpenTelemetry TracerProvider and provide context-propagation
// helpers, so that a service can call:
//
//	_ = trace.Init(trace.Config{AppName: "checkout"})
//	defer trace.Shutdown()
//
// and let the log package automatically attach trace_id / span_id to every
// log line emitted via log.Info(ctx, ...). This is the reason the convenience
// wrappers in the log package already accept a context.Context — once trace
// lands, no caller-side change is needed to light up correlation.
//
// TODO:
//   - Pick a default exporter (OTLP HTTP is the current candidate).
//   - Provide a gin / net/http middleware that starts spans automatically.
//   - Wire AGENDA_TRACE_ENDPOINT / AGENDA_TRACE_SAMPLE_RATIO env vars.
package trace
