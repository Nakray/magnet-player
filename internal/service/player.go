package service

import (
	"log"
	"time"

	"github.com/Nakray/magnet-player/internal/storage"
	e "github.com/Nakray/magnet-player/internal/torrent"
	"github.com/anacrolix/torrent"
)

// PlayerService - это Use Case. Он знает "КАК" работает наше приложение.
type PlayerService struct {
	engine *e.Engine
	metaDB *storage.MetadataDB
	cache  *storage.CacheManager
}

func NewPlayerService(eng *e.Engine, db *storage.MetadataDB, cache *storage.CacheManager) *PlayerService {
	return &PlayerService{
		engine: eng,
		metaDB: db,
		cache:  cache,
	}
}

func (s *PlayerService) ProcessMagnet(magnetLink string) (string, error) {
	t, err := s.engine.AddMagnet(magnetLink)
	if err != nil {
		return "", err
	}

	// запускаем фоновую закгрузку
	go s.processDownload(t)

	return t.InfoHash().String(), nil
}

func (s *PlayerService) processDownload(t *torrent.Torrent) {
	select {
	case <-t.GotInfo():
		// OK
	case <-time.After(60 * time.Second):
		log.Printf("Timeout fetching metadata for %s, dropping", t.InfoHash())
		t.Drop()
		return
	}

	// Включаем скачивание в движке
	t.DownloadAll()

	for _, f := range t.Files() {
		meta := &storage.FileMeta{
			Hash:       t.InfoHash().String(),
			Path:       s.cache.GetAbsPath(f),
			Size:       f.Length(),
			LastAccess: time.Now(),
		}

		if err := s.metaDB.Save(meta); err != nil {
			log.Printf("Service: failed to save meta: %v", err)
		}
		s.cache.Add(meta)
	}

	log.Printf("Service: Torrent %s processed", t.Name())
}
