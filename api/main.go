package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/asymmetric-effort/convocate/internal/ac"
	"github.com/asymmetric-effort/convocate/internal/amgr"
	"github.com/asymmetric-effort/convocate/internal/auth"
	"github.com/asymmetric-effort/convocate/internal/events"
	"github.com/asymmetric-effort/convocate/internal/ide"
	"github.com/asymmetric-effort/convocate/internal/middleware"
	"github.com/asymmetric-effort/convocate/internal/nmgr"
	"github.com/asymmetric-effort/convocate/internal/pb"
	"github.com/asymmetric-effort/convocate/internal/repo"
	"github.com/asymmetric-effort/convocate/internal/status"
	"github.com/asymmetric-effort/convocate/internal/sup"
)

func main() {
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

	addr := ":8443"
	fmt.Printf("Convocate API server listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
