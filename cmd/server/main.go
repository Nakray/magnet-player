package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Nakray/magnet-player/internal/config"
	"github.com/Nakray/magnet-player/internal/httpserver"
	"github.com/Nakray/magnet-player/internal/storage"
	"github.com/Nakray/magnet-player/internal/torrent"
)

func main() {
	cfgPath := os.Getenv("MP_CONFIG")
	if cfgPath == "" {
		cfgPath = "./config.json"
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := os.MkdirAll(cfg.Storage.BaseDir, 0o755); err != nil {
		log.Fatalf("create base dir: %v", err)
	}

	metaDB, err := storage.NewMetadataDB(cfg.Storage.DbPath)
	if err != nil {
		log.Fatalf("open metadata db: %v", err)
	}
	defer metaDB.Close()

	cacheMgr := storage.NewCacheManager(cfg.Storage.MaxSizeGB)

	storedFiles, err := metaDB.GetAllFiles()
	if err != nil {
		log.Printf("WARNING: failed to load metadata: %v", err)
	} else {
		cacheMgr.RestoreState(storedFiles)
		log.Printf("Cache restored: %d files, total size: %.2f GB",
			len(storedFiles),
			float64(cacheMgr.CurrentSize())/(1<<30),
		)
	}

	engine, err := torrent.NewEngine(cfg.Storage.BaseDir)
	if err != nil {
		log.Fatalf("init torrent engine: %v", err)
	}
	defer engine.Close()

	router := httpserver.NewRouter(engine, cacheMgr, metaDB)

	addr := ":8080"
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
