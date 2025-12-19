package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Nakray/magnet-player/internal/httpserver"
	"github.com/Nakray/magnet-player/internal/storage"
	"github.com/Nakray/magnet-player/internal/torrent"
)

func main() {
	// Конфиг бери из env для простоты
	baseDir := getEnv("MP_BASE_DIR", "./data")
	dbPath := getEnv("MP_DB_PATH", "./data/meta.db")
	maxSizeGB := int64(10) // TODO: читать из env/флага

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		log.Fatalf("create base dir: %v", err)
	}

	// init storage (BoltDB + кеш)
	metaDB, err := storage.NewMetadataDB(dbPath)
	if err != nil {
		log.Fatalf("open metadata db: %v", err)
	}
	defer metaDB.Close()

	cacheMgr := storage.NewCacheManager(maxSizeGB)
	// TODO: восстановить состояние из metaDB, обновить cacheMgr.currentSize

	// init torrent engine
	engine, err := torrent.NewEngine(baseDir)
	if err != nil {
		log.Fatalf("init torrent engine: %v", err)
	}
	defer engine.Close()

	// init HTTP router
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
