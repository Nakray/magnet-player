package storage

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

var ErrFileNotFound = errors.New("file not found")

type FileMeta struct {
	Hash       string    `json:"hash"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	LastAccess time.Time `json:"last_access"`
}

type CacheManager struct {
	mu          sync.Mutex
	maxSize     int64
	currentSize int64
	baseDir     string
	files       map[string]*FileMeta // hash: file
}

func NewCacheManager(maxSizeGB int64, baseDir string) *CacheManager {
	var max int64
	if maxSizeGB <= 0 {
		max = -1
	} else {
		max = maxSizeGB * (1 << 30) //10 GB
	}
	return &CacheManager{
		maxSize: max,
		files:   make(map[string]*FileMeta),
		baseDir: baseDir,
	}
}

func (c *CacheManager) CurrentSize() int64 {
	return c.currentSize
}

func (c *CacheManager) BaseDir() string {
	return c.baseDir
}

func (c *CacheManager) ReserveSpace(required int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.maxSize <= 0 {
		return nil
	}
	if required > c.maxSize {
		return errors.New("required size exceeds max cache size")
	}

	for c.currentSize+required > c.maxSize {
		victim := c.getLRUVictim()
		if victim == nil {
			return errors.New("no victim for eviction")
		}
		if err := c.evictLocked(victim); err != nil {
			return err
		}
	}
	return nil
}

func (c *CacheManager) getLRUVictim() *FileMeta {
	var v *FileMeta
	for _, m := range c.files {
		if v == nil || m.LastAccess.Before(v.LastAccess) {
			v = m
		}
	}
	return v
}

func (c *CacheManager) evictLocked(v *FileMeta) error {
	if err := os.RemoveAll(v.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	c.currentSize -= v.Size
	delete(c.files, v.Hash)
	return nil
}

func (c *CacheManager) Touch(hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok := c.files[hash]; ok {
		m.LastAccess = time.Now()
	}
}

func (c *CacheManager) Add(file *FileMeta) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.files[file.Hash] = file
}

func (c *CacheManager) RestoreState(files []*FileMeta) []*FileMeta {
	c.mu.Lock()
	defer c.mu.Unlock()

	var badFiles []*FileMeta

	for _, f := range files {
		info, err := os.Stat(f.Path)
		if err != nil {
			badFiles = append(badFiles, f)
			continue
		}

		f.Size = info.Size()
		c.files[f.Hash] = f
		c.currentSize += f.Size
	}
	return badFiles
}

func (c *CacheManager) GetAbsPath(file *torrent.File) string {
	return filepath.Join(c.baseDir, file.Path())
}

// GetAllFiles возвращает список всех файлов в кеше
func (c *CacheManager) GetAllFiles() []*FileMeta {
	c.mu.Lock()
	defer c.mu.Unlock()

	files := make([]*FileMeta, 0, len(c.files))
	for _, f := range c.files {
		files = append(files, f)
	}
	return files
}

// GetFile возвращает файл по хешу
func (c *CacheManager) GetFile(hash string) *FileMeta {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.files[hash]
}

// Remove удаляет файл из кеша
func (c *CacheManager) Remove(hash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, ok := c.files[hash]
	if !ok {
		return errors.New("file not found in cache")
	}

	if err := os.RemoveAll(f.Path); err != nil && !os.IsNotExist(err) {
		return err
	}

	c.currentSize -= f.Size
	delete(c.files, hash)
	return nil
}
