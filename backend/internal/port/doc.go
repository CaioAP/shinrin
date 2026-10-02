// Package port declares the hexagon's boundaries as Go interfaces.
//
// Driving ports (driving.go) are the use cases the application offers. Inbound
// adapters such as the HTTP API depend on them, application services in
// internal/app implement them.
//
// Driven ports (driven.go) are what the application needs from the outside
// world: storage, market data providers, LLM providers, the clock. Application
// services depend on them, outbound adapters in internal/adapter/out implement
// them.
//
// Ports only mention domain types and the standard library. Keep each
// interface small (interface segregation): a consumer should never depend on
// a method it does not call.
package port
