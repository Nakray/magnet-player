package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Nakray/magnet-player/internal/config"
	"github.com/Nakray/magnet-player/internal/httpserver"
	"github.com/Nakray/magnet-player/internal/service"
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

	cacheMgr := storage.NewCacheManager(cfg.Storage.MaxSizeGB, cfg.Storage.BaseDir)

	restoreState(metaDB, cacheMgr)

	engine, err := torrent.NewEngine()
	if err != nil {
		log.Fatalf("init torrent engine: %v", err)
	}
	defer engine.Close()

	player := service.NewPlayerService(engine, metaDB, cacheMgr)

	router := httpserver.NewRouter(player)

	addr := ":8080"
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func restoreState(metaDB *storage.MetadataDB, cacheMgr *storage.CacheManager) {
	var badFiles []*storage.FileMeta
	storedFiles, err := metaDB.GetAllFiles()
	if err != nil {
		log.Printf("WARNING: failed to load metadata: %v", err)
	} else {
		badFiles = cacheMgr.RestoreState(storedFiles)
		log.Printf("Cache restored: %d files, total size: %.2f GB",
			len(storedFiles),
			float64(cacheMgr.CurrentSize())/(1<<30),
		)
	}

	for _, f := range badFiles {
		if err := metaDB.Remove(f); err != nil {
			log.Printf("WARNING: failed to delete file from db: %v", err)
		}
	}
}
