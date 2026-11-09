package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

func gatewayHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	body, err := io.ReadAll(io.LimitReader(r.Body, 20<<20))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	state := getSessionState(r)
	features := extractFeatures(r, body, state)

	if benchMode == "gateway_only" {
		writeBenchmarkHeaders(w, start, nil)
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"mode":   "gateway_only",
		})
		return
	}

	trace := predictWithLeavesTimed(features)

	if debugLogs {
		fmt.Printf("[PREDICT] method=%s path=%s class=%s score=%.6f blocked=%v features=%v\n",
			r.Method, r.URL.Path, trace.Prediction.ClassName, trace.Prediction.Score,
			trace.Prediction.Blocked, featuresToSlice(features))
	}

	if benchmarkHeaders {
		w.Header().Set("X-Api-Shield-Class", trace.Prediction.ClassName)
		w.Header().Set("X-Api-Shield-Score", fmt.Sprintf("%.6f", trace.Prediction.Score))
		w.Header().Set("X-Api-Shield-Blocked", strconv.FormatBool(trace.Prediction.Blocked && blockingEnabled))
		w.Header().Set("X-Model-Inference-Us", strconv.FormatInt(trace.Inference.Microseconds(), 10))
		w.Header().Set("X-Model-Total-Us", strconv.FormatInt(trace.TotalModel.Microseconds(), 10))
	}

	if trace.Prediction.Blocked && blockingEnabled {
		writeBenchmarkHeaders(w, start, &trace)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"status": "blocked",
			"reason": "request classified as suspicious by API Shield Gateway",
			"class":  trace.Prediction.ClassName,
			"score":  trace.Prediction.Score,
		})
		return
	}

	if benchMode == "gateway_ml" {
		writeBenchmarkHeaders(w, start, &trace)
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"mode":   "gateway_ml",
			"class":  trace.Prediction.ClassName,
			"score":  trace.Prediction.Score,
		})
		return
	}

	proxyResp, err := proxyToTarget(r, body)
	if err != nil {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}

	writeBenchmarkHeaders(w, start, &trace)
	copyHeaders(w.Header(), proxyResp.Header)
	w.WriteHeader(proxyResp.Status)
	_, _ = w.Write(proxyResp.Body)
}
func writeBenchmarkHeaders(w http.ResponseWriter, start time.Time, trace *PredictionTrace) {
	if !benchmarkHeaders {
		return
	}

	w.Header().Set("X-Gateway-Total-Us", strconv.FormatInt(time.Since(start).Microseconds(), 10))

	if trace != nil {
		w.Header().Set("X-Model-Inference-Us", strconv.FormatInt(trace.Inference.Microseconds(), 10))
		w.Header().Set("X-Model-Total-Us", strconv.FormatInt(trace.TotalModel.Microseconds(), 10))
		w.Header().Set("X-Api-Shield-Class", trace.Prediction.ClassName)
		w.Header().Set("X-Api-Shield-Score", fmt.Sprintf("%.6f", trace.Prediction.Score))
		w.Header().Set("X-Api-Shield-Blocked", strconv.FormatBool(trace.Prediction.Blocked && blockingEnabled))
	}
}
func writeJSON(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(obj)
}
