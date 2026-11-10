package main

import (
	"net/http"
	"testing"
)

func TestResearchSessionIDNeverReachesBackend(t *testing.T) {
	requestHeaders := make(http.Header)
	requestHeaders.Set("X-Research-Session-ID", "11111111-1111-4111-8111-111111111111")
	requestHeaders.Set("Authorization", "Bearer example-token")
	outbound := make(http.Header)
	copyHeaders(outbound, requestHeaders)
	if outbound.Get("X-Research-Session-ID") != "" {
		t.Fatal("research session ID was forwarded to backend")
	}
	if outbound.Get("Authorization") != "Bearer example-token" {
		t.Fatal("normal authentication was not preserved")
	}
}
