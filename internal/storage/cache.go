package storage

import (
	"errors"
	"os"
	"sync"
	"time"
)

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
	files       map[string]*FileMeta
}

func NewCacheManager(maxSizeGB int64) *CacheManager {
	var max int64
	if maxSizeGB <= 0 {
		max = -1
	} else {
		max = maxSizeGB * 1024 * 1024 * 1024
	}
	return &CacheManager{
		maxSize: max,
		files:   make(map[string]*FileMeta),
	}
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
