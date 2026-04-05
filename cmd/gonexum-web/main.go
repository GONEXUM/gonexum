package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed static
var staticFiles embed.FS

// AppVersion est injecté au build via -ldflags "-X main.AppVersion=x.x.x"
var AppVersion = "dev"

func main() {
	flagPort   := flag.Int("port", 8080, "Port d'écoute HTTP")
	flagHost   := flag.String("host", "127.0.0.1", "Adresse d'écoute (0.0.0.0 pour accès réseau)")
	flagConfig := flag.String("config", "", "Chemin vers settings.json")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `GONEXUM Web — Interface web pour création et upload de torrents

Usage:
  gonexum-web [options]

Options:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Config:
  Les paramètres (API key, passkey, etc.) sont lus depuis:
    Linux/seedbox : ~/.config/GONEXUM/settings.json
    macOS         : ~/Library/Application Support/GONEXUM/settings.json

Exemples:
  gonexum-web                          # localhost:8080
  gonexum-web -port 9090               # localhost:9090
  gonexum-web -host 0.0.0.0 -port 8080 # accessible sur le réseau local
`)
	}

	flag.Parse()

	if *flagConfig != "" {
		configFilePath = *flagConfig
	}

	// API routes
	http.HandleFunc("/api/settings", handleSettings)
	http.HandleFunc("/api/browse", handleBrowse)
	http.HandleFunc("/api/mediainfo", handleMediaInfo)
	http.HandleFunc("/api/tmdb/search", handleTMDBSearch)
	http.HandleFunc("/api/tmdb/details", handleTMDBDetails)
	http.HandleFunc("/api/process", handleProcess)
	http.HandleFunc("/api/events", handleEvents)
	http.HandleFunc("/api/nfo/validate", handleNFOValidate)
	http.HandleFunc("/api/nfo/preview", handleNFOPreview)
	http.HandleFunc("/api/categories", handleCategories)
	http.HandleFunc("/api/queue", handleQueue)
	http.HandleFunc("/api/queue/remove", handleQueueRemove)
	http.HandleFunc("/api/queue/clear", handleQueueClear)
	http.HandleFunc("/api/queue/events", handleQueueEvents)

	// Static files (embedded)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf("%s:%d", *flagHost, *flagPort)
	fmt.Printf("╔══════════════════════════════════════╗\n")
	fmt.Printf("║      GONEXUM Web %-18s ║\n", AppVersion)
	fmt.Printf("╚══════════════════════════════════════╝\n\n")
	fmt.Printf("  Serveur démarré sur http://%s\n", addr)
	if *flagHost == "0.0.0.0" {
		fmt.Printf("  Accessible depuis le réseau local\n")
	}
	fmt.Printf("  Arrêt: Ctrl+C\n\n")

	log.Fatal(http.ListenAndServe(addr, nil))
}
