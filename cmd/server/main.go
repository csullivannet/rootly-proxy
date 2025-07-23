package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/csullivannet/rootly-proxy/internal/database"
	"github.com/csullivannet/rootly-proxy/internal/handler"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// generateSelfSignedCert generates a self-signed certificate for localhost testing
func generateSelfSignedCert() (tls.Certificate, error) {
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}, nil
}

func setupHTTPServer(certManager *autocert.Manager) *http.Server {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "80"
	}
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle /ready endpoint
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Service is ready"))
			return
		}

		// Handle ACME challenges
		if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			certManager.HTTPHandler(nil).ServeHTTP(w, r)
			return
		}

		// Redirect everything else to HTTPS
		httpsURL := "https://" + r.Host + r.URL.Path
		if r.URL.RawQuery != "" {
			httpsURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, httpsURL, http.StatusFound)
	})

	return &http.Server{
		Addr:    ":" + port,
		Handler: httpHandler,
	}
}

func setupHTTPSServer(proxyHandler http.Handler, certManager *autocert.Manager, hostnames []string) *http.Server {
	port := os.Getenv("HTTPS_PORT")
	if port == "" {
		port = "443"
	}

	// Generate self-signed certificate for localhost (for testing)
	localhostCert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("Failed to generate localhost certificate: %v", err)
	}

	// Certificates will be fetched on-demand to avoid blocking startup
	log.Printf("Certificates will be fetched on-demand for hostnames: %v", hostnames)
	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			log.Printf("GetCertificate called for: %s", hello.ServerName)

			// Use self-signed certificate for localhost
			if hello.ServerName == "localhost" || hello.ServerName == "127.0.0.1" {
				log.Printf("Using self-signed certificate for %s", hello.ServerName)
				return &localhostCert, nil
			}

			// Use ACME certificate for other domains
			cert, err := certManager.GetCertificate(hello)
			if err != nil {
				log.Printf("Error getting certificate for %s: %v", hello.ServerName, err)
				// Return self-signed certificate for unknown domains
				log.Printf("Using self-signed certificate for unknown domain %s", hello.ServerName)
				return &localhostCert, nil
			}
			log.Printf("Successfully got certificate for %s", hello.ServerName)
			return cert, nil
		},
		MinVersion: tls.VersionTLS12,
	}

	return &http.Server{
		Addr:         ":" + port,
		Handler:      proxyHandler,
		TLSConfig:    tlsConfig,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func setupHealthServer() *http.Server {
	port := os.Getenv("HEALTH_PORT")
	if port == "" {
		port = "8443"
	}
	healthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Service is ready"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	return &http.Server{
		Addr:    ":" + port,
		Handler: healthHandler,
	}
}

func main() {
	// Setup database connection
	db := database.SetupDatabase()
	defer db.Close()

	// Create repository
	repo := database.NewPostgresRepository(db)

	// Create proxy handler
	proxyHandler := handler.NewProxyHandler(repo)

	// Get all hostnames from database for ACME configuration
	hostnames, err := database.GetAllHostnames(repo)
	if err != nil {
		log.Fatalf("Failed to get hostnames from database: %v", err)
	}
	log.Printf("Found hostnames in database: %v", hostnames)

	// ACME manager configured for Pebble
	pebbleURL := os.Getenv("PEBBLE_URL")
	if pebbleURL == "" {
		pebbleURL = "https://localhost:14000/dir" // Default Pebble directory URL
	}

	certManager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache("/app/certs"),
		HostPolicy: autocert.HostWhitelist(hostnames...),
	}

	certManager.Client = &acme.Client{
		DirectoryURL: pebbleURL,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Skip certificate verification for Pebble
				},
			},
		},
	}

	// Setup HTTP server
	httpSrv := setupHTTPServer(certManager)

	go func() {
		log.Printf("Starting HTTP server for ACME challenges on :%s...", httpSrv.Addr[1:])
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on HTTP %s: %v", httpSrv.Addr, err)
		}
	}()

	// Give HTTP server time to start
	time.Sleep(1 * time.Second)

	// Setup health server
	healthSrv := setupHealthServer()

	go func() {
		log.Printf("Starting health check server on %s...", healthSrv.Addr)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on health port %s: %v", healthSrv.Addr, err)
		}
	}()

	// Setup HTTPS server (includes certificate pre-fetching)
	srv := setupHTTPSServer(proxyHandler, certManager, hostnames)
	log.Printf("Server is starting on %s...", srv.Addr)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v", srv.Addr, err)
	}

	// Graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := healthSrv.Shutdown(ctx); err != nil {
			log.Printf("Could not gracefully shutdown the Health server: %v", err)
		}
		if err := httpSrv.Shutdown(ctx); err != nil {
			log.Printf("Could not gracefully shutdown the HTTP server: %v", err)
		}
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown the HTTPS server: %v", err)
		}
		close(done)
	}()

	<-done
	log.Println("Server stopped")
}
