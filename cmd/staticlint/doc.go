// Command staticlint implements a multichecker for static analysis
// of the go-musthave-metrics project.
//
// A multichecker is a driver that runs a set of go/analysis analyzers
// over a set of packages, reporting diagnostics in the familiar
// <file>:<line>:<column>: <message> format (the same as go vet).
//
// # Included analyzers
//
// The checker includes the following groups of analyzers:
//
//   - standard analyzers from golang.org/x/tools/go/analysis/passes;
//   - all SA analyzers from Staticcheck, which detect bugs, incorrect
//     API usage and performance problems;
//   - the ST1000, ST1005 and ST1020 analyzers from the Stylecheck class:
//     ST1000 checks package comments, ST1005 checks error strings,
//     and ST1020 checks documentation of exported functions;
//   - errcheck, which detects ignored errors returned by functions;
//   - bodyclose, which detects HTTP response bodies that are not closed;
//   - osexit, a custom analyzer that prohibits direct os.Exit calls from
//     the main function of package main (see the osexit subpackage).
//
// # Running the checker
//
// Run the checker from the project root over all packages:
//
//	go run ./cmd/staticlint ./...
//
// Arguments are packages, exactly like go vet. Individual packages or
// package patterns can be passed instead:
//
//	go run ./cmd/staticlint ./internal/... ./cmd/server
//
// Alternatively, build a standalone binary and run it:
//
//	go build -o staticlint ./cmd/staticlint
//	./staticlint ./...
//
// # Managing analyzers
//
// Run the following to see the complete list of registered analyzers and
// their flags:
//
//	go run ./cmd/staticlint -help
//
// Any analyzer can be enabled or disabled individually through the flags
// that multichecker generates for it:
//
//	go run ./cmd/staticlint -<analyzer>.disable=false ./...
package main
