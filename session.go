package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

var sessionStore sync.Map

type SessionState struct {
	mu                  sync.Mutex
	RequestNumber       int
	EndpointAccessCount map[string]int
	EndpointTargets     map[string]map[string]bool
	LastTarget          string
	TargetEvents        int
	TargetChanges       int
}

func updateBehavioralState(state *SessionState, template string, targetID string) (int, float64, float64) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.EndpointAccessCount == nil {
		state.EndpointAccessCount = make(map[string]int)
	}
	if state.EndpointTargets == nil {
		state.EndpointTargets = make(map[string]map[string]bool)
	}
	if targetID == "" {
		targetID = "none"
	}

	state.RequestNumber++
	state.EndpointAccessCount[template]++

	repeatedCount := state.EndpointAccessCount[template]
	if repeatedCount > 100 {
		repeatedCount = 100
	}

	// Python adds the normalized target to the endpoint set even when it is "none".
	if state.EndpointTargets[template] == nil {
		state.EndpointTargets[template] = make(map[string]bool)
	}
	state.EndpointTargets[template][targetID] = true

	probeRate := float64(len(state.EndpointTargets[template])) / float64(state.EndpointAccessCount[template])
	if probeRate > 1 {
		probeRate = 1
	}

	// Python compares every current target with the previous request target,
	// including "none", then divides cumulative changes by request count so far.
	if state.RequestNumber > 1 && state.LastTarget != targetID {
		state.TargetChanges++
	}
	state.LastTarget = targetID

	targetChangeRate := float64(state.TargetChanges) / float64(state.RequestNumber)
	if targetChangeRate > 1 {
		targetChangeRate = 1
	}

	return repeatedCount, probeRate, targetChangeRate
}
func getSessionState(r *http.Request) *SessionState {
	key := sessionKey(r)

	val, _ := sessionStore.LoadOrStore(key, &SessionState{
		EndpointAccessCount: make(map[string]int),
		EndpointTargets:     make(map[string]map[string]bool),
	})

	return val.(*SessionState)
}
func sessionKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.TrimSpace(auth) != "" {
		return auth
	}

	if cookie := r.Header.Get("Cookie"); strings.TrimSpace(cookie) != "" {
		return cookie
	}

	if xff := r.Header.Get("X-Forwarded-For"); strings.TrimSpace(xff) != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}

	return "anonymous"
}
