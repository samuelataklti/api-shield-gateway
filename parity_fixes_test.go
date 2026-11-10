package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const testSessionA = "11111111-1111-4111-8111-111111111111"
const testSessionB = "22222222-2222-4222-8222-222222222222"

type parityFixture struct {
	ID       string    `json:"id"`
	SID      string    `json:"sid"`
	Auth     string    `json:"auth"`
	Method   string    `json:"method"`
	URL      string    `json:"url"`
	Body     string    `json:"body"`
	Expected []float64 `json:"expected"`
}

func testRequest(method, url, sid, auth string) *http.Request {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+url, nil)
	r.RemoteAddr = "127.0.0.1:12345"
	if sid != "" {
		r.Header.Set("X-Research-Session-ID", sid)
	}
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	return r
}

func TestPythonGoStrict12ParityWithResearchSession(t *testing.T) {
	t.Setenv("RESEARCH_PARITY_MODE", "1")
	contents, err := os.ReadFile("strict12_parity_fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []parityFixture
	if err := json.Unmarshal(contents, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 9 {
		t.Fatalf("expected 9 independent Python-generated fixture rows, got %d", len(fixtures))
	}

	for _, fixture := range fixtures {
		r := testRequest(fixture.Method, fixture.URL, fixture.SID, fixture.Auth)
		state := getSessionState(r)
		f := extractFeatures(r, []byte(fixture.Body), state)
		actual := []float64{
			f.LogReqSize, f.IsReadMethod, f.IsWriteMethod, f.EndpointDepth,
			f.EndpointHasIDPlaceholder, f.PayloadPresent, f.LogPayloadLength,
			f.PayloadShapeScore, f.PayloadNonAlnumRatio,
			f.RepeatedEndpointAccessCountSoFar, f.ResourceProbeRateSoFar, f.TargetChangeRateSoFar,
		}
		if len(fixture.Expected) != len(actual) {
			t.Fatalf("%s: wrong feature count", fixture.ID)
		}
		for i := range actual {
			if math.Abs(actual[i]-fixture.Expected[i]) > 1e-9 {
				t.Errorf("%s: feature %d differs (Go %v, Python %v)", fixture.ID, i+1, actual[i], fixture.Expected[i])
			}
		}
	}
}

func TestTemplateDoesNotMergeUnrelatedQueries(t *testing.T) {
	withPagination := testRequest(http.MethodGet, "/workshop/api/shop/products?limit=30&offset=0", "", "")
	withoutPagination := testRequest(http.MethodGet, "/workshop/api/shop/products?sort=recent", "", "")
	if got := endpointTemplateForRequest(withPagination); got != "/workshop/api/shop/products?limit={n}&offset={n}" {
		t.Fatalf("unexpected training template: %s", got)
	}
	if got := endpointTemplateForRequest(withoutPagination); got != "/workshop/api/shop/products" {
		t.Fatalf("unrecognized query must keep path-only template: %s", got)
	}
}

func TestResearchHeaderDisabledByDefault(t *testing.T) {
	t.Setenv("RESEARCH_PARITY_MODE", "0")
	r := testRequest(http.MethodGet, "/api/v1/items", testSessionA, "Bearer separate-user")
	if got := sessionKey(r); got != "Bearer separate-user" {
		t.Fatalf("disabled research header altered normal authentication behavior: %s", got)
	}
}

func TestResearchHeaderCannotOverrideRemoteOrInvalidRequest(t *testing.T) {
	t.Setenv("RESEARCH_PARITY_MODE", "1")
	r := testRequest(http.MethodGet, "/api/v1/items", testSessionA, "Bearer separate-user")
	r.RemoteAddr = "198.51.100.17:12345"
	if got := sessionKey(r); got != "Bearer separate-user" {
		t.Fatalf("remote client overrode token with research header: %s", got)
	}
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Research-Session-ID", "not-a-uuid")
	if got := sessionKey(r); got != "Bearer separate-user" {
		t.Fatalf("invalid research ID overrode authentication key: %s", got)
	}
}

func TestTwoResearchSessionIDsNeverMerge(t *testing.T) {
	t.Setenv("RESEARCH_PARITY_MODE", "1")
	a := testRequest(http.MethodGet, "/api/v1/items", testSessionA, "Bearer same-demo-token")
	b := testRequest(http.MethodGet, "/api/v1/items", testSessionB, "Bearer same-demo-token")
	if sessionKey(a) == sessionKey(b) {
		t.Fatal("distinct research session UUIDs unexpectedly map to the same key")
	}
}
