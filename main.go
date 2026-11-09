package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

func main() {
	var err error

	targetURL, err = url.Parse(getenv("TARGET_URL", defaultTarget))
	if err != nil {
		log.Fatalf("Invalid TARGET_URL: %v", err)
	}

	maxProcs := getenvInt("GOMAXPROCS", runtime.NumCPU())
	if maxProcs > 0 {
		runtime.GOMAXPROCS(maxProcs)
	}

	debugLogs = getenv("DEBUG_LOGS", "0") == "1"
	benchmarkHeaders = getenv("BENCHMARK_HEADERS", "1") == "1"
	blockingEnabled = getenv("BLOCKING_ENABLED", "1") == "1"

	benchMode = strings.ToLower(strings.TrimSpace(getenv("BENCH_MODE", "full")))
	if benchMode != "gateway_only" && benchMode != "gateway_ml" && benchMode != "full" {
		log.Fatalf("Invalid BENCH_MODE=%q. Use gateway_only, gateway_ml, or full", benchMode)
	}

	setupHTTPClient()
	setupLightGBMLeaves()

	mux := http.NewServeMux()
	mux.HandleFunc("/__benchmark/gateway", benchmarkGatewayOnly)
	mux.HandleFunc("/__benchmark/model", benchmarkModelOnly)
	mux.HandleFunc("/", gatewayHandler)

	fmt.Println("--- API SHIELD GATEWAY: FINAL STRICT-12 LIGHTGBM MODE ---")
	fmt.Printf("[+] Listening on http://localhost%s\n", defaultListen)
	fmt.Printf("[+] Proxy target: %s\n", targetURL.String())
	fmt.Printf("[+] Feature count: %d\n", featureCount)
	fmt.Printf("[+] BENCH_MODE=%s blocking=%v debug_logs=%v benchmark_headers=%v\n",
		benchMode, blockingEnabled, debugLogs, benchmarkHeaders)
	fmt.Printf("[+] GOMAXPROCS=%d\n", runtime.GOMAXPROCS(0))

	server := &http.Server{
		Addr:              defaultListen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
