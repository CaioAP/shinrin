// Package app groups the application services, one subpackage per use-case
// area (system, catalog; later ingest, scoring, report). Each service
// implements a driving port from internal/port, depends only on domain and
// port, and receives its driven ports through its constructor.
package app
