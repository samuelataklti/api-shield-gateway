package main

import (
	"net/http"
	"strconv"
	"time"
)

func benchmarkGatewayOnly(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Gateway-Only-Latency-Us", strconv.FormatInt(time.Since(start).Microseconds(), 10))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"benchmark": "gateway_only",
	})
}
func benchmarkModelOnly(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	f := Features{
		LogReqSize:                       5.5,
		IsReadMethod:                     1,
		IsWriteMethod:                    0,
		EndpointDepth:                    6,
		EndpointHasIDPlaceholder:         1,
		PayloadPresent:                   0,
		LogPayloadLength:                 0,
		PayloadShapeScore:                0,
		PayloadNonAlnumRatio:             0.10,
		RepeatedEndpointAccessCountSoFar: 3,
		ResourceProbeRateSoFar:           0.50,
		TargetChangeRateSoFar:            0.40,
	}

	trace := predictWithLeavesTimed(f)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Model-Only-Latency-Us", strconv.FormatInt(time.Since(start).Microseconds(), 10))
	w.Header().Set("X-Model-Inference-Us", strconv.FormatInt(trace.Inference.Microseconds(), 10))
	w.Header().Set("X-Model-Total-Us", strconv.FormatInt(trace.TotalModel.Microseconds(), 10))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"benchmark":  "model_only",
		"prediction": trace.Prediction,
	})
}
