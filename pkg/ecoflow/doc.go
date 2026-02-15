// Package ecoflow provides a resilient, signed HTTP client for the EcoFlow
// developer API.
//
// The package focuses on production behavior:
//   - HMAC request signing
//   - tuned HTTP transport defaults
//   - retry with jittered backoff
//   - environment/.env-driven configuration
//   - structured logging and observability hooks
package ecoflow
