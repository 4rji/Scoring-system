package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rootInfo struct {
	Endpoints []string `json:"endpoints"`
}

type checkPayload struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type checkResult struct {
	Endpoint string `json:"endpoint"`
	OK       bool   `json:"ok"`
	Detail   string `json:"detail"`
	Status   int    `json:"status"`
	Error    string `json:"error,omitempty"`
}

type aggregateResponse struct {
	OK        bool          `json:"ok"`
	Source    string        `json:"source"`
	FetchedAt time.Time     `json:"fetched_at"`
	Checks    []checkResult `json:"checks"`
}

var defaultEndpoints = []string{
	"/check/http",
	"/check/https",
	"/check/smtp",
	"/check/pop3",
	"/check/ftp",
	"/check/dns",
}

func main() {
	baseURL := strings.TrimSuffix(env("SCORING_BASE_URL", "http://localhost:8080"), "/")
	bindAddr := env("READER_ADDR", ":9090")
	timeout := time.Duration(envInt("READER_TIMEOUT_SECONDS", 5)) * time.Second

	client := &http.Client{Timeout: timeout}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		endpoints := fetchEndpoints(r.Context(), client, baseURL)
		if len(endpoints) == 0 {
			respondJSON(w, http.StatusServiceUnavailable, map[string]any{
				"ok":     false,
				"detail": "no endpoints discovered",
			})
			return
		}

		checks := collectChecks(r.Context(), client, baseURL, endpoints)
		agg := aggregateResponse{
			Source:    baseURL,
			FetchedAt: time.Now().UTC(),
			Checks:    checks,
		}
		agg.OK = allChecksOK(checks)

		status := http.StatusOK
		if !agg.OK {
			status = http.StatusServiceUnavailable
		}
		respondJSON(w, status, agg)
	})

	log.Printf("reader listening on %s and polling %s (timeout %s)", bindAddr, baseURL, timeout)
	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatalf("reader server error: %v", err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return def
}

func fetchEndpoints(ctx context.Context, client *http.Client, base string) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		log.Printf("endpoint discovery request build failed: %v", err)
		return defaultEndpoints
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("endpoint discovery failed: %v (using defaults)", err)
		return defaultEndpoints
	}
	defer resp.Body.Close()

	var info rootInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		log.Printf("endpoint discovery decode failed: %v (using defaults)", err)
		return defaultEndpoints
	}

	if len(info.Endpoints) == 0 {
		log.Printf("endpoint discovery returned empty list (using defaults)")
		return defaultEndpoints
	}

	return info.Endpoints
}

func collectChecks(ctx context.Context, client *http.Client, base string, endpoints []string) []checkResult {
	results := make([]checkResult, len(endpoints))
	var wg sync.WaitGroup
	wg.Add(len(endpoints))

	for i, endpoint := range endpoints {
		i, endpoint := i, ensureLeadingSlash(endpoint)
		go func() {
			defer wg.Done()
			results[i] = poll(ctx, client, base, endpoint)
		}()
	}

	wg.Wait()
	return results
}

func poll(ctx context.Context, client *http.Client, base, endpoint string) checkResult {
	fullURL := base + endpoint
	res := checkResult{Endpoint: endpoint}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		res.Error = fmt.Sprintf("request build error: %v", err)
		return res
	}

	resp, err := client.Do(req)
	if err != nil {
		res.Error = fmt.Sprintf("request error: %v", err)
		return res
	}
	defer resp.Body.Close()

	res.Status = resp.StatusCode

	var payload checkPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		res.Error = fmt.Sprintf("decode error: %v", err)
		res.Detail = http.StatusText(resp.StatusCode)
		res.OK = resp.StatusCode >= 200 && resp.StatusCode < 300
		return res
	}

	res.OK = payload.OK
	res.Detail = payload.Detail

	if resp.StatusCode >= 400 && res.OK {
		res.OK = false
		if res.Detail == "" {
			res.Detail = resp.Status
		}
	}

	return res
}

func allChecksOK(checks []checkResult) bool {
	for _, c := range checks {
		if !c.OK {
			return false
		}
	}
	return true
}

func ensureLeadingSlash(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("response encode error: %v", err)
	}
}
