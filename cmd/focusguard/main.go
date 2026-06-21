// Command focusguard is a local website blocker: it serves a web UI on localhost and, based on the activated sites, redirects their domains to 127.0.0.1 by editing the operating system's hosts file.

// It requires administrator/root privileges to be able to write to the hosts file (on Linux/Windows). See README.md for installation as a service.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/ezerevello/focusguard/internal/api"
	"github.com/ezerevello/focusguard/internal/store"
	"github.com/ezerevello/focusguard/internal/sysutil"
)

//go:embed web/static/*
var staticFiles embed.FS

func main() {
	port := flag.Int("port", 7878, "port where the local web UI is served")
	dataPath := flag.String("data", "", "path to the state file (default: user data directory)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser automatically on startup")
	flag.Parse()

	if *dataPath == "" {
		*dataPath = filepath.Join(defaultDataDir(), "data.json")
	}

	st, err := store.Open(*dataPath)
	if err != nil {
		log.Fatalf("could not open data store at %s: %v", *dataPath, err)
	}

	if !sysutil.IsElevated() {
		log.Printf("WARNING: this process does not have administrator/root privileges.")
		log.Printf("You will be able to use the web UI, but the actual blocking (editing the hosts file) will fail.")
		log.Printf("Run with sudo (Linux) or as Administrator (Windows), or install the service: see README.md")
	}

	engine := api.New(st)

	mux := http.NewServeMux()
	staticSub, err := fs.Sub(staticFiles, "web/static")
	if err != nil {
		log.Fatal(err)
	}
	indexHTML, err := fs.ReadFile(staticSub, "index.html")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		// We serve the index directly from memory (instead of delegating to
		// FileServer) to avoid the "index.html -> ./" redirect that
		// http.FileServer automatically performs, which would cause a loop here.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.Handle("/api/", engine.Router())

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	url := fmt.Sprintf("http://%s", addr)
	log.Printf("FocusGuard v%s listening on %s (data in %s)", api.Version, url, *dataPath)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	if !*noBrowser {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(url)
		}()
	}

	waitForShutdown(srv)
}

func defaultDataDir() string {
	if runtime.GOOS == "windows" {
		pd := os.Getenv("ProgramData")
		if pd == "" {
			pd = `C:\ProgramData`
		}
		return filepath.Join(pd, "FocusGuard")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".focusguard")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func waitForShutdown(srv *http.Server) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Println("shutting down FocusGuard...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
