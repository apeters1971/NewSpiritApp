package main

import (
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apeters/newspirit/internal/api"
	"github.com/apeters/newspirit/internal/hub"
	"github.com/apeters/newspirit/internal/store"
	"github.com/apeters/newspirit/web"
)

func main() {
	certFile := flag.String("cert", os.Getenv("TLS_CERT"), "TLS certificate PEM; enables HTTPS on :8443")
	keyFile := flag.String("key", os.Getenv("TLS_KEY"), "TLS private key PEM (defaults next to -cert)")
	flag.Parse()

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
	cert := strings.TrimSpace(*certFile)
	key := strings.TrimSpace(*keyFile)
	tls := cert != ""
	if tls && key == "" {
		key = inferKey(cert)
		if key == "" {
			log.Fatal("TLS key is required: pass -key or set TLS_KEY")
		}
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		if tls {
			addr = ":8443"
		} else {
			addr = ":8080"
		}
	}
	scheme := "http"
	if tls {
		scheme = "https"
	}
	log.Printf("New Spirit listening on %s", addr)
	if tls {
		log.Printf("TLS certificate: %s", cert)
	}
	log.Printf("member UI:      %s://localhost%s/", scheme, addr)
	log.Printf("controller UI:  %s://localhost%s/controller", scheme, addr)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if tls {
		err = httpSrv.ListenAndServeTLS(cert, key)
	} else {
		err = httpSrv.ListenAndServe()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func inferKey(cert string) string {
	ext := filepath.Ext(cert)
	base := strings.TrimSuffix(cert, ext)
	dir := filepath.Dir(cert)
	candidates := []string{
		base + ".key",
		base + "-key" + ext,
		filepath.Join(dir, "key.pem"),
		filepath.Join(dir, "privkey.pem"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
