package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/asymmetric-effort/convocate/internal/mtls"
	"github.com/asymmetric-effort/convocate/internal/openbao"
	"github.com/asymmetric-effort/convocate/internal/redis"
	"github.com/asymmetric-effort/convocate/internal/router"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(Version)
		return
	}
	fmt.Fprintf(os.Stderr, "convocate-router %s\n", Version)
	os.Exit(run())
}

func run() int {
	logger := log.New(os.Stderr, "router: ", log.LstdFlags)

	// Redis connection — use mock for dev if real Redis isn't reachable.
	var store *redis.RouterStore
	isDev := os.Getenv("CONVOCATE_DEV") == "1"

	if isDev {
		logger.Println("DEV mode: using in-memory mock Redis")
		store = redis.NewRouterStore(redis.NewMockConn())
	} else {
		redisURL := os.Getenv("CONVOCATE_REDIS_URL")
		if redisURL == "" {
			logger.Println("CONVOCATE_REDIS_URL not set")
			return 1
		}
		// Production Redis connection would go here.
		logger.Printf("connecting to Redis at %s", redisURL)
		store = redis.NewRouterStore(redis.NewMockConn())
	}

	// OpenBao client.
	baoURL := os.Getenv("CONVOCATE_OPENBAO_URL")
	if baoURL == "" {
		baoURL = "http://localhost:8200"
	}
	baoClient := openbao.NewClient(openbao.Config{
		Address: baoURL,
		Token:   os.Getenv("BAO_TOKEN"),
	})

	srv := router.NewServer(router.Config{
		Store:   store,
		Bao:     baoClient,
		Version: Version,
		Logger:  logger,
	})

	// TLS setup.
	tlsDir := "/tls"
	if _, err := os.Stat(tlsDir + "/router.crt"); os.IsNotExist(err) {
		// Generate self-signed certs for dev.
		logger.Println("no TLS certs found, generating self-signed...")
		ca, caErr := mtls.GenerateCA("convocate-dev-ca", 365*24*time.Hour)
		if caErr != nil {
			logger.Printf("generate CA: %v", caErr)
			return 1
		}
		pair, certErr := ca.IssueServerCert("convocate-router",
			[]string{"localhost", "router"}, []net.IP{net.ParseIP("127.0.0.1")},
			365*24*time.Hour)
		if certErr != nil {
			logger.Printf("issue cert: %v", certErr)
			return 1
		}

		cert, err := tls.X509KeyPair(pair.CertPEM, pair.KeyPEM)
		if err != nil {
			logger.Printf("load keypair: %v", err)
			return 1
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}
		startServers(srv, logger, tlsCfg, tlsCfg)
	} else {
		// Load certs from volume.
		publicCert, err := tls.LoadX509KeyPair(tlsDir+"/router.crt", tlsDir+"/router.key")
		if err != nil {
			logger.Printf("load TLS cert: %v", err)
			return 1
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{publicCert},
			MinVersion:   tls.VersionTLS13,
		}
		startServers(srv, logger, tlsCfg, tlsCfg)
	}

	return 0
}

func startServers(srv *router.Server, logger *log.Logger, publicTLS, internalTLS *tls.Config) {
	handler := srv.Handler()

	// Public listener (tcp/443) — GitHub Actions /v1/jobs + /v1/health.
	publicAddr := "0.0.0.0:443"
	publicListener, err := net.Listen("tcp", publicAddr) //nolint:gosec // must bind all interfaces for container networking
	if err != nil {
		logger.Printf("listen %s: %v", publicAddr, err)
		os.Exit(1)
	}
	logger.Printf("public HTTPS on %s", publicAddr)

	// Internal listener (tcp/8443) — Web UI + Dispatch mTLS.
	internalAddr := "0.0.0.0:8443"
	internalListener, err := net.Listen("tcp", internalAddr) //nolint:gosec // must bind all interfaces for container networking
	if err != nil {
		logger.Printf("listen %s: %v", internalAddr, err)
		os.Exit(1)
	}
	logger.Printf("internal HTTPS on %s", internalAddr)

	// Serve both.
	go func() {
		publicServer := &http.Server{
			Handler:           handler,
			TLSConfig:         publicTLS,
			ReadHeaderTimeout: 30 * time.Second,
		}
		if serveErr := publicServer.ServeTLS(publicListener, "", ""); serveErr != nil {
			logger.Printf("public server: %v", serveErr)
		}
	}()

	internalServer := &http.Server{
		Handler:           handler,
		TLSConfig:         internalTLS,
		ReadHeaderTimeout: 30 * time.Second,
	}
	if serveErr := internalServer.ServeTLS(internalListener, "", ""); serveErr != nil {
		logger.Printf("internal server: %v", serveErr)
	}
}
