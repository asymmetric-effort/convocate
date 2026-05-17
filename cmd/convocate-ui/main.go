package main

import (
	"crypto/tls"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/asymmetric-effort/convocate/internal/mtls"
	"github.com/asymmetric-effort/convocate/internal/webui"
)

const caCertPath = "/tls/ca.crt"

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(Version)
		return
	}
	fmt.Fprintf(os.Stderr, "convocate-ui %s\n", Version)
	os.Exit(run())
}

func run() int {
	logger := log.New(os.Stderr, "ui: ", log.LstdFlags)

	routerAPIURL := os.Getenv("CONVOCATE_ROUTER_API_URL")
	if routerAPIURL == "" {
		routerAPIURL = "https://router-api:443"
	}

	target, err := url.Parse(routerAPIURL)
	if err != nil {
		logger.Printf("invalid CONVOCATE_ROUTER_API_URL: %v", err)
		return 1
	}

	// Build the TLS transport for the reverse proxy.
	// Prefer the CA trust bundle at /tls/ca.crt for proper verification.
	// Fall back to InsecureSkipVerify only when the CA file is absent (dev mode).
	proxyTransport, transportErr := buildProxyTransport(logger)
	if transportErr != nil {
		logger.Printf("build proxy transport: %v", transportErr)
		return 1
	}

	// Build the reverse proxy for API requests to router-api.
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		Transport: proxyTransport,
	}

	// Serve embedded Web UI static files.
	distFS, distErr := fs.Sub(webui.Dist, "dist")
	if distErr != nil {
		logger.Printf("webui dist not available: %v", distErr)
	}
	fileServer := http.FileServer(http.FS(distFS))

	mux := http.NewServeMux()

	// Proxy /ui/api/* to router-api.
	mux.HandleFunc("/ui/api/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// Proxy /auth/* to router-api.
	mux.HandleFunc("/auth/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// Proxy /v1/health to router-api (for dashboard health check).
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// Health endpoint for the UI container itself.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"convocate-ui"}`))
	})

	// Static files (SPA with index.html fallback).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// API paths that aren't handled above get 404.
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}
		// Try serving a static file.
		if distErr == nil {
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			}
			if _, statErr := fs.Stat(distFS, strings.TrimPrefix(path, "/")); statErr == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// SPA fallback: serve index.html for unmatched paths.
		if distErr == nil {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// TLS setup.
	tlsDir := "/tls"
	var tlsCfg *tls.Config

	if _, statErr := os.Stat(tlsDir + "/router.crt"); os.IsNotExist(statErr) {
		// Generate self-signed certs for dev.
		logger.Println("no TLS certs found, generating self-signed...")
		ca, caErr := mtls.GenerateCA("convocate-dev-ca", 365*24*time.Hour)
		if caErr != nil {
			logger.Printf("generate CA: %v", caErr)
			return 1
		}
		dnsNames := []string{"localhost", "ui", "convocate-ui"}
		pair, certErr := ca.IssueServerCert("convocate-ui",
			dnsNames, []net.IP{net.ParseIP("127.0.0.1")},
			365*24*time.Hour)
		if certErr != nil {
			logger.Printf("issue cert: %v", certErr)
			return 1
		}
		cert, loadErr := tls.X509KeyPair(pair.CertPEM, pair.KeyPEM)
		if loadErr != nil {
			logger.Printf("load keypair: %v", loadErr)
			return 1
		}
		tlsCfg = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}
	} else {
		// Load certs from volume (reuse router cert for simplicity in dev).
		cert, loadErr := tls.LoadX509KeyPair(tlsDir+"/router.crt", tlsDir+"/router.key")
		if loadErr != nil {
			logger.Printf("load TLS cert: %v", loadErr)
			return 1
		}
		tlsCfg = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}
	}

	// Wrap handler with debug logging if CONVOCATE_DEBUG=1.
	var handler http.Handler = mux
	if os.Getenv("CONVOCATE_DEBUG") == "1" {
		logger.Println("DEBUG mode: logging all HTTP requests")
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: 200}
			mux.ServeHTTP(rw, r)
			logger.Printf("DEBUG %s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
		})
	}

	addr := "0.0.0.0:443"
	listener, listenErr := net.Listen("tcp", addr) //nolint:gosec // must bind all interfaces for container networking
	if listenErr != nil {
		logger.Printf("listen %s: %v", addr, listenErr)
		return 1
	}
	logger.Printf("HTTPS on %s (proxying API to %s)", addr, routerAPIURL)

	server := &http.Server{
		Handler:           handler,
		TLSConfig:         tlsCfg,
		ReadHeaderTimeout: 30 * time.Second,
	}
	if serveErr := server.ServeTLS(listener, "", ""); serveErr != nil {
		logger.Printf("server: %v", serveErr)
		return 1
	}
	return 0
}

// buildProxyTransport returns an http.Transport whose TLS config trusts the
// CA bundle at /tls/ca.crt when available. If the file does not exist (dev /
// self-signed mode) it falls back to InsecureSkipVerify and logs a warning.
func buildProxyTransport(logger *log.Logger) (*http.Transport, error) {
	caPEM, readErr := os.ReadFile(caCertPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			logger.Printf("WARNING: %s not found – proxy using InsecureSkipVerify (dev mode only)", caCertPath)
			return &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec // Dev-mode fallback: CA bundle absent.
				},
			}, nil
		}
		return nil, fmt.Errorf("read %s: %w", caCertPath, readErr)
	}

	tlsCfg, tlsErr := mtls.PlainTLSConfig(caPEM)
	if tlsErr != nil {
		return nil, fmt.Errorf("build TLS config from %s: %w", caCertPath, tlsErr)
	}
	return &http.Transport{TLSClientConfig: tlsCfg}, nil
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
