// Package testfx provides test-only helpers that sit between httptest.Server
// and Okta adapter tests (TESTING §4.2).
//
// It is a normal Go package (not a _test.go file) so service, TUI, and
// integration tests can all import it. The production entry point (cmd/ota)
// must never import this package — enforced by golangci-lint depguard
// (TECH_STACK §9.2, CONVENTIONS §2).
//
// Contents:
//   - FakeServer — httptest.Server wrapper that dispatches requests against a
//     scenario JSON (see testdata/scenarios/*.json).
//   - LoadFixture / LoadFixtureMeta — read a JSON body and its .meta.json sidecar.
//   - LoadHTTPResponse — compose an *http.Response for errormap tests.
package testfx
