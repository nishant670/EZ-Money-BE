package http

import (
	"sort"
	"strings"
	"testing"
)

// TestEverySessionRouteSkipsTheStaticBearer is the guard the prefix list never
// had.
//
// The static bearer runs before routing, as global middleware, and answers 401
// on anything `skipsStaticBearer` does not name. That makes the failure silent
// in exactly the way that matters: the handler is correct, the session token is
// valid, the test suite passes, and the app gets 401 from every environment
// where AUTH_BEARER is configured. It has now happened twice — the recurring
// candidate endpoints (found in M1, broken from the day they shipped) and
// subscription occurrences, push registration, uploads and reports (found here,
// by probing the running server rather than by reading the list).
//
// Walking the registered routes is what makes this stick. A new endpoint is
// covered the moment it is registered, with no one having to remember a file
// two hundred lines away from the one they are editing.
func TestEverySessionRouteSkipsTheStaticBearer(t *testing.T) {
	var gated []string

	for route := range registeredRouteSet(t) {
		parts := strings.SplitN(route, " ", 2)
		if len(parts) != 2 {
			continue
		}
		path := parts[1]
		if !strings.HasPrefix(path, "/v1/") || deliberatelyGuarded(path) {
			continue
		}
		if !skipsStaticBearer(path) {
			gated = append(gated, route)
		}
	}

	if len(gated) > 0 {
		sort.Strings(gated)
		t.Fatalf(
			"these routes are behind the static bearer, so the app answers 401 wherever AUTH_BEARER is set.\n"+
				"Add the prefix to skipsStaticBearer, or add it to staticBearerGuardedPrefixes if the gate is intended:\n%s",
			strings.Join(gated, "\n"),
		)
	}
}

func deliberatelyGuarded(path string) bool {
	for _, prefix := range staticBearerGuardedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// The four found by probing a running backend. Named individually so a
// regression names the feature it breaks rather than a path.
func TestPreviouslyGatedRoutesAreReachable(t *testing.T) {
	cases := []struct {
		path    string
		feature string
	}{
		{"/v1/push-devices", "push registration — without it no device token is ever stored"},
		{"/v1/subscription-occurrences", "the autopay review card's Confirm and Correct/revert"},
		{"/v1/upload", "receipt attachments"},
		{"/v1/reports/transactions/summary", "the transaction summary report"},
		{"/v1/monthly-review", "the monthly review screen"},
	}
	for _, testCase := range cases {
		if !skipsStaticBearer(testCase.path) {
			t.Errorf("%s is gated by the static bearer, which breaks %s", testCase.path, testCase.feature)
		}
	}
}

// The gate is only meaningful if something is actually behind it, and the one
// prefix left carries its own bearer. This does not assert that the situation
// is right — it asserts that it is *visible*, so the question gets asked rather
// than inherited.
func TestTheStaticBearerGuardsOnlySeparatelyAuthenticatedRoutes(t *testing.T) {
	for _, prefix := range staticBearerGuardedPrefixes {
		if prefix != "/v1/admin" {
			t.Fatalf(
				"%s is guarded by the static bearer alone. Confirm that is intended before adding it here.",
				prefix,
			)
		}
	}
}
