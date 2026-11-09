package main

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultListen = ":8080"
	defaultTarget = "http://localhost:8888"
	featureCount  = 12
	labelBenign   = 0
	labelBOLA     = 1
	labelBFLA     = 2
	labelSQLi     = 3
)

var (
	debugLogs        bool
	benchmarkHeaders bool
	benchMode        string
	blockingEnabled  bool
)

func getenv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}

	return n
}
func getenvFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}

	return n
}
