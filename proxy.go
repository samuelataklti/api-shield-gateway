package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	targetURL   *url.URL
	proxyClient *http.Client
)

type ProxyResponse struct {
	Status  int
	Header  http.Header
	Body    []byte
	Latency time.Duration
}

func setupHTTPClient() {
	maxIdle := getenvInt("PROXY_MAX_IDLE_CONNS", 4096)
	maxIdlePerHost := getenvInt("PROXY_MAX_IDLE_CONNS_PER_HOST", 4096)
	maxConnsPerHost := getenvInt("PROXY_MAX_CONNS_PER_HOST", 0)

	proxyClient = &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     false,
			MaxIdleConns:          maxIdle,
			MaxIdleConnsPerHost:   maxIdlePerHost,
			MaxConnsPerHost:       maxConnsPerHost,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}
func proxyToTarget(r *http.Request, body []byte) (*ProxyResponse, error) {
	target := *targetURL
	target.Path = joinPaths(targetURL.Path, r.URL.Path)
	target.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		target.String(),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	copyHeaders(req.Header, r.Header)
	req.Host = target.Host

	start := time.Now()

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, err
	}

	return &ProxyResponse{
		Status:  resp.StatusCode,
		Header:  resp.Header.Clone(),
		Body:    respBody,
		Latency: time.Since(start),
	}, nil
}
func copyHeaders(dst, src http.Header) {
	for k, values := range src {
		lower := strings.ToLower(k)
		if lower == "connection" ||
			lower == "keep-alive" ||
			lower == "proxy-authenticate" ||
			lower == "proxy-authorization" ||
			lower == "te" ||
			lower == "trailer" ||
			lower == "transfer-encoding" ||
			lower == "upgrade" {
			continue
		}

		for _, v := range values {
			dst.Add(k, v)
		}
	}
}
func joinPaths(a, b string) string {
	if a == "" || a == "/" {
		if strings.HasPrefix(b, "/") {
			return b
		}
		return "/" + b
	}

	return strings.TrimRight(a, "/") + "/" + strings.TrimLeft(b, "/")
}
