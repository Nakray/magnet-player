package service

import (
	"io"
	"log"
	"os"
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

// GetFileReader возвращает reader для файла по хешу
func (s *PlayerService) GetFileMeta(hash string) *storage.FileMeta {
	meta := s.cache.GetFile(hash)
	if meta != nil {
		s.cache.Touch(hash)
	}
	return meta
}

// GetFileReader возвращает reader для файла по хешу
func (s *PlayerService) GetFileReader(hash string) (io.ReadCloser, int64, error) {
	meta := s.cache.GetFile(hash)
	if meta == nil {
		return nil, 0, storage.ErrFileNotFound
	}

	s.cache.Touch(hash)

	file, err := os.Open(meta.Path)
	if err != nil {
		return nil, 0, err
	}

	return file, meta.Size, nil
}

// GetFileReaderWithRange возвращает reader для файла с поддержкой Range
func (s *PlayerService) GetFileReaderWithRange(hash string, start, end int64) (io.ReadCloser, int64, error) {
	meta := s.cache.GetFile(hash)
	if meta == nil {
		return nil, 0, storage.ErrFileNotFound
	}

	s.cache.Touch(hash)

	file, err := os.Open(meta.Path)
	if err != nil {
		return nil, 0, err
	}

	if start > 0 {
		if _, err := file.Seek(start, 0); err != nil {
			file.Close()
			return nil, 0, err
		}
	}

	size := meta.Size - start
	if end > 0 && end < size {
		size = end - start + 1
	}

	return &rangeReader{file: file, remaining: size}, size, nil
}

// GetAllFiles возвращает список всех файлов в кеше
func (s *PlayerService) GetAllFiles() []*storage.FileMeta {
	return s.cache.GetAllFiles()
}

// RemoveFile удаляет файл из кеша и БД
func (s *PlayerService) RemoveFile(hash string) error {
	meta := s.cache.GetFile(hash)
	if meta == nil {
		return storage.ErrFileNotFound
	}

	if err := s.cache.Remove(hash); err != nil {
		return err
	}

	return s.metaDB.RemoveByHash(hash)
}

// rangeReader обёртка для чтения с ограничением по байтам
type rangeReader struct {
	file      *os.File
	remaining int64
}

func (r *rangeReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.file.Read(p)
	r.remaining -= int64(n)
	return n, err
}

func (r *rangeReader) Close() error {
	return r.file.Close()
}
