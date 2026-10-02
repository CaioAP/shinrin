// Package adapter holds the hexagon's adapters.
//
//	adapter/in/   driving adapters: translate an outside request (HTTP, CLI,
//	              a scheduled job) into a call on a driving port.
//	adapter/out/  driven adapters: implement a driven port against a concrete
//	              technology (Postgres, a provider's HTTP API, an LLM vendor).
//
// Adapters may import domain, port and their own third-party libraries. They
// never import internal/app or each other; wiring happens in cmd/.
package adapter
