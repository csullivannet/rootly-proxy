package handler

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/csullivannet/rootly-proxy/internal/database"
)

type ProxyHandler struct {
	repository database.Repository
	httpClient *http.Client
}

func NewProxyHandler(repo database.Repository) *ProxyHandler {
	// Configure HTTP client with timeouts and connection pooling
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
	}

	return &ProxyHandler{
		repository: repo,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := r.Host
	log.Printf("Handling request for hostname: %s", hostname)

	statusPage, err := h.repository.FindByHostname(hostname)
	if err != nil {
		log.Printf("Database error for hostname %s: %v", hostname, err)
		http.Error(w, "502 - Bad Gateway", http.StatusBadGateway)
		return
	}

	if statusPage == nil {
		log.Printf("No status page found for hostname: %s", hostname)
		http.Error(w, "404 - Domain not found", http.StatusNotFound)
		return
	}

	log.Printf("Fetching content from backend URL: %s for hostname: %s", statusPage.PageDataURL, hostname)

	// Retry logic for backend connections
	var resp *http.Response
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = h.httpClient.Get(statusPage.PageDataURL)
		if err == nil {
			break
		}

		log.Printf("Attempt %d/%d failed for backend URL %s: %v", attempt+1, maxRetries, statusPage.PageDataURL, err)
		if attempt < maxRetries-1 {
			// Exponential backoff: 100ms, 200ms, 400ms
			backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
			log.Printf("Retrying in %v...", backoff)
			time.Sleep(backoff)
		}
	}

	if err != nil {
		log.Printf("All retry attempts failed for backend URL %s: %v", statusPage.PageDataURL, err)
		http.Error(w, "502 - Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Backend returned non-200 status code %d for URL: %s", resp.StatusCode, statusPage.PageDataURL)
		http.Error(w, "502 - Bad Gateway", http.StatusBadGateway)
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	bytesWritten, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
	} else {
		log.Printf("Successfully proxied %d bytes for hostname: %s", bytesWritten, hostname)
	}
}
