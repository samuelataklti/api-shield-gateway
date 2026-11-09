package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	idLikeRe      = regexp.MustCompile(`(?i)^[a-f0-9]{8,}$|^[0-9]+$|^[a-f0-9-]{12,}$`)
	sqlKeywordRe  = regexp.MustCompile(`(?i)\b(select|union|insert|update|delete|drop|alter|where|or|and|sleep|benchmark|information_schema)\b`)
	encodedCharRe = regexp.MustCompile(`%[0-9a-fA-F]{2}`)
)

type Features struct {
	LogReqSize                       float64
	IsReadMethod                     float64
	IsWriteMethod                    float64
	EndpointDepth                    float64
	EndpointHasIDPlaceholder         float64
	PayloadPresent                   float64
	LogPayloadLength                 float64
	PayloadShapeScore                float64
	PayloadNonAlnumRatio             float64
	RepeatedEndpointAccessCountSoFar float64
	ResourceProbeRateSoFar           float64
	TargetChangeRateSoFar            float64
}

func extractFeatures(r *http.Request, body []byte, state *SessionState) Features {
	path := r.URL.Path
	method := strings.ToUpper(r.Method)
	template := endpointTemplate(path)
	targetID := extractTargetID(path, r.URL.RawQuery, body)

	// Match the final k6 dataset collector: req_size was request-body length.
	reqSize := len(body)

	// Match final Python raw_payload semantics: body only, with no query concatenation.
	payload := string(body)
	payloadLength := utf8.RuneCountInString(payload)
	if payloadLength > 10000 {
		payloadLength = 10000
	}

	endpointDepth := countEndpointDepth(path)
	if endpointDepth > 10 {
		endpointDepth = 10
	}

	// Offline endpoint_has_id_placeholder checked the endpoint template structure.
	hasIDPlaceholder := strings.Contains(template, "{") && strings.Contains(template, "}")

	// Offline is_read_method was GET only.
	isRead := method == http.MethodGet
	isWrite := method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete

	repeatedCount, probeRate, targetChangeRate := updateBehavioralState(state, template, targetID)

	return Features{
		LogReqSize:                       math.Log1p(float64(reqSize)),
		IsReadMethod:                     boolFloat(isRead),
		IsWriteMethod:                    boolFloat(isWrite),
		EndpointDepth:                    float64(endpointDepth),
		EndpointHasIDPlaceholder:         boolFloat(hasIDPlaceholder),
		PayloadPresent:                   boolFloat(payloadLength > 0),
		LogPayloadLength:                 math.Log1p(float64(payloadLength)),
		PayloadShapeScore:                payloadShapeScore(payload),
		PayloadNonAlnumRatio:             nonAlnumRatio(payload),
		RepeatedEndpointAccessCountSoFar: float64(repeatedCount),
		ResourceProbeRateSoFar:           probeRate,
		TargetChangeRateSoFar:            targetChangeRate,
	}
}
func payloadForAnalysis(query string, body []byte) string {
	var parts []string

	if query != "" {
		values, err := url.ParseQuery(query)
		if err == nil {
			for _, vals := range values {
				for _, v := range vals {
					parts = append(parts, v)
				}
			}
		} else {
			parts = append(parts, query)
		}
	}

	if len(body) > 0 {
		var obj any
		if json.Unmarshal(body, &obj) == nil {
			collectJSONValues(obj, &parts)
		} else {
			parts = append(parts, string(body))
		}
	}

	return strings.Join(parts, " ")
}
func collectJSONValues(v any, parts *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for _, val := range x {
			collectJSONValues(val, parts)
		}
	case []any:
		for _, val := range x {
			collectJSONValues(val, parts)
		}
	case string:
		*parts = append(*parts, x)
	case float64:
		*parts = append(*parts, strconv.FormatFloat(x, 'f', -1, 64))
	case bool:
		*parts = append(*parts, strconv.FormatBool(x))
	}
}
func estimateRequestSize(r *http.Request, body []byte) int {
	// The final anti-overfit dataset's req_size was produced by k6 reqSize(body).
	return len(body)
}
func countEndpointDepth(path string) int {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	count := 0
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	return count
}
func endpointHasID(path string, targetID string) bool {
	if targetID != "" && targetID != "none" {
		return true
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, p := range parts {
		if isIDLike(p) {
			return true
		}
	}

	return false
}
func endpointTemplate(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "/"
	}

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if isIDLike(p) {
			out = append(out, "{id}")
		} else {
			out = append(out, strings.ToLower(p))
		}
	}

	if len(out) == 0 {
		return "/"
	}

	return "/" + strings.Join(out, "/")
}
func extractTargetID(path string, query string, body []byte) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, p := range parts {
		if isIDLike(p) {
			return p
		}
	}

	values, err := url.ParseQuery(query)
	if err == nil {
		for _, key := range []string{"id", "vehicle_id", "vehicleId", "video_id", "videoId", "order_id", "orderId", "uuid"} {
			if v := values.Get(key); v != "" {
				return v
			}
		}
	}

	if len(body) > 0 && len(body) < 2<<20 {
		var obj any
		if json.Unmarshal(body, &obj) == nil {
			if id := findIDInJSON(obj); id != "" {
				return id
			}
		}
	}

	return "none"
}
func findIDInJSON(v any) string {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"id", "vehicle_id", "vehicleId", "video_id", "videoId", "order_id", "orderId", "uuid"} {
			if val, ok := x[key]; ok {
				if s, ok := toString(val); ok && s != "" {
					return s
				}
			}
		}
		for _, child := range x {
			if id := findIDInJSON(child); id != "" {
				return id
			}
		}
	case []any:
		for _, child := range x {
			if id := findIDInJSON(child); id != "" {
				return id
			}
		}
	}
	return ""
}
func isIDLike(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	if idLikeRe.MatchString(s) {
		return true
	}

	if len(s) >= 6 {
		hasDigit := false
		hasLetter := false
		for _, ch := range s {
			if ch >= '0' && ch <= '9' {
				hasDigit = true
			}
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
				hasLetter = true
			}
		}
		return hasDigit && hasLetter
	}

	return false
}
func payloadShapeScore(payload string) float64 {
	if payload == "" {
		return 0
	}

	quoteCount := strings.Count(payload, "'") + strings.Count(payload, `"`)
	if quoteCount > 10 {
		quoteCount = 10
	}

	operatorCount := 0
	for _, token := range []string{"=", "<", ">", "-", "/", "*", "%"} {
		operatorCount += strings.Count(payload, token)
	}
	if operatorCount > 20 {
		operatorCount = 20
	}

	symbolCount := 0
	for _, ch := range payload {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && !unicode.IsSpace(ch) {
			symbolCount++
		}
	}
	if symbolCount > 30 {
		symbolCount = 30
	}

	entropy := entropyLight(payload)
	if entropy > 8 {
		entropy = 8
	}

	score := float64(quoteCount)*0.8 + float64(operatorCount)*0.4 + float64(symbolCount)*0.2 + entropy
	if score > 20 {
		return 20
	}
	return score
}
func entropyLight(s string) float64 {
	if s == "" {
		return 0
	}
	counts := make(map[rune]int)
	total := 0
	for _, ch := range s {
		counts[ch]++
		total++
	}
	if total == 0 {
		return 0
	}
	ent := 0.0
	for _, count := range counts {
		p := float64(count) / float64(total)
		ent -= p * math.Log2(p)
	}
	return ent
}
func nonAlnumRatio(s string) float64 {
	if s == "" {
		return 0
	}

	payloadLength := utf8.RuneCountInString(s)
	if payloadLength > 10000 {
		payloadLength = 10000
	}
	if payloadLength == 0 {
		return 0
	}

	symbolCount := 0
	for _, ch := range s {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && !unicode.IsSpace(ch) {
			symbolCount++
		}
	}

	ratio := float64(symbolCount) / float64(payloadLength)
	if ratio > 1 {
		return 1
	}
	return ratio
}
