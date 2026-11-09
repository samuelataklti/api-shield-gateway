package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func near(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

func TestStrict12EmptyGetAndSessionState(t *testing.T) {
	state := &SessionState{}
	r1 := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/videos/100", nil)
	f1 := extractFeatures(r1, nil, state)
	if f1.IsReadMethod != 1 || f1.IsWriteMethod != 0 || f1.PayloadPresent != 0 {
		t.Fatalf("unexpected GET flags: %+v", f1)
	}
	if f1.LogReqSize != 0 || f1.LogPayloadLength != 0 {
		t.Fatalf("empty body must have zero log sizes: %+v", f1)
	}
	if f1.EndpointDepth != 4 || f1.EndpointHasIDPlaceholder != 1 {
		t.Fatalf("unexpected endpoint structure: %+v", f1)
	}
	if f1.RepeatedEndpointAccessCountSoFar != 1 || !near(f1.ResourceProbeRateSoFar, 1.0) || f1.TargetChangeRateSoFar != 0 {
		t.Fatalf("unexpected first-request state: %+v", f1)
	}

	r2 := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/videos/101", nil)
	f2 := extractFeatures(r2, nil, state)
	if f2.RepeatedEndpointAccessCountSoFar != 2 || !near(f2.ResourceProbeRateSoFar, 1.0) || !near(f2.TargetChangeRateSoFar, 0.5) {
		t.Fatalf("unexpected second-request state: %+v", f2)
	}
}

func TestStrict12PayloadFeatures(t *testing.T) {
	body := []byte(`{"query":"' OR 1=1 --"}`)
	r := httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/search", nil)
	f := extractFeatures(r, body, &SessionState{})
	if f.IsReadMethod != 0 || f.IsWriteMethod != 1 || f.PayloadPresent != 1 {
		t.Fatalf("unexpected POST flags: %+v", f)
	}
	if !near(f.LogReqSize, math.Log1p(float64(len(body)))) || !near(f.LogPayloadLength, math.Log1p(float64(len(body)))) {
		t.Fatalf("unexpected payload/request size: %+v", f)
	}
	if f.PayloadShapeScore <= 0 || f.PayloadNonAlnumRatio <= 0 || f.PayloadNonAlnumRatio > 1 {
		t.Fatalf("unexpected payload shape/ratio: %+v", f)
	}
}

func TestBlockingPolicy(t *testing.T) {
	t.Setenv("THRESHOLD_BOLA", "0.50")
	t.Setenv("THRESHOLD_BFLA", "0.50")
	t.Setenv("THRESHOLD_SQLI", "0.50")
	if shouldBlock(labelBenign, 0.99) {
		t.Fatal("benign label must not be blocked")
	}
	if shouldBlock(labelBOLA, 0.49) || !shouldBlock(labelBOLA, 0.50) || !shouldBlock(labelSQLi, 0.75) {
		t.Fatal("default attack-class block thresholds changed")
	}
	t.Setenv("THRESHOLD_BOLA", "0.80")
	if shouldBlock(labelBOLA, 0.70) || !shouldBlock(labelBOLA, 0.90) {
		t.Fatal("BOLA threshold override changed")
	}
}

func TestConcurrentSessionUpdates(t *testing.T) {
	const requests = 200
	state := &SessionState{}
	var wg sync.WaitGroup
	wg.Add(requests)
	for i := 0; i < requests; i++ {
		go func() {
			defer wg.Done()
			updateBehavioralState(state, "/api/v1/videos/{id}", "100")
		}()
	}
	wg.Wait()
	if state.RequestNumber != requests || state.EndpointAccessCount["/api/v1/videos/{id}"] != requests {
		t.Fatalf("lost concurrent session updates: requests=%d endpoint=%d", state.RequestNumber, state.EndpointAccessCount["/api/v1/videos/{id}"])
	}
}
