package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/apeters/newspirit/internal/api"
	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
	"github.com/apeters/newspirit/web"
)

func main() {
	dataDir := env("DATA_DIR", "data")
	secret := os.Getenv("CONTROLLER_SECRET")
	if secret == "" {
		log.Println("warning: CONTROLLER_SECRET is empty; controller login will be unavailable")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	st, err := store.Open(filepath.Join(dataDir, "spirit.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	clientFS, err := fs.Sub(web.Client, "client")
	if err != nil {
		log.Fatal(err)
	}
	controllerFS, err := fs.Sub(web.Controller, "controller")
	if err != nil {
		log.Fatal(err)
	}

	srv := api.New(st, hub.New(), secret, clientFS, controllerFS)
	addr := env("ADDR", ":8080")
	log.Printf("New Spirit listening on %s", addr)
	log.Printf("member UI:      http://localhost%s/", addr)
	log.Printf("controller UI:  http://localhost%s/controller", addr)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
