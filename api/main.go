package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/asymmetric-effort/convocate/internal/ac"
	"github.com/asymmetric-effort/convocate/internal/amgr"
	"github.com/asymmetric-effort/convocate/internal/auth"
	"github.com/asymmetric-effort/convocate/internal/db"
	"github.com/asymmetric-effort/convocate/internal/events"
	"github.com/asymmetric-effort/convocate/internal/ide"
	"github.com/asymmetric-effort/convocate/internal/k8s"
	"github.com/asymmetric-effort/convocate/internal/llm"
	"github.com/asymmetric-effort/convocate/internal/middleware"
	"github.com/asymmetric-effort/convocate/internal/nmgr"
	"github.com/asymmetric-effort/convocate/internal/pb"
	"github.com/asymmetric-effort/convocate/internal/repo"
	"github.com/asymmetric-effort/convocate/internal/status"
	"github.com/asymmetric-effort/convocate/internal/sup"
)

func main() {
	auth.InitJWT()
	llm.Init()

	if err := db.InitPostgres(); err != nil {
		log.Printf("WARNING: PostgreSQL unavailable: %v (using mock stores)", err)
	} else {
		if err := db.RunMigrations(); err != nil {
			log.Printf("WARNING: migration failed: %v", err)
		}
	}

	if err := db.InitRedis(); err != nil {
		log.Printf("WARNING: Redis unavailable: %v", err)
	}

	if err := k8s.Init(); err != nil {
		log.Printf("WARNING: K8s API unavailable: %v (node/agent management disabled)", err)
	} else {
		if err := k8s.EnsureAgentNamespace(context.Background()); err != nil {
			log.Printf("WARNING: could not ensure agent namespace: %v", err)
		}
	}

	mux := http.NewServeMux()

	status.Register(mux)
	auth.Register(mux)
	nmgr.Register(mux)
	amgr.Register(mux)
	pb.Register(mux)
	ide.Register(mux)
	repo.Register(mux)
	ac.Register(mux)
	sup.Register(mux)
	events.Register(mux)

	handler := middleware.CORS(mux)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8443"
	}

	fmt.Printf("Convocate API server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
