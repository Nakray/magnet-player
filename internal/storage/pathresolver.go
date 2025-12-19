package storage

import (
	"fmt"
	"path/filepath"
	"strings"
)

type FolderMode int

const (
	ModeStructured FolderMode = iota
	ModeFlat
)

type PathConfig struct {
	BaseDir    string
	FolderMode FolderMode
}

func ResolvePath(cfg PathConfig, torrentName, innerPath, hash string) string {
	switch cfg.FolderMode {
	case ModeStructured:
		return filepath.Join(cfg.BaseDir, innerPath)
	case ModeFlat:
		name := filepath.Base(innerPath)
		safe := fmt.Sprintf("%s_%s_%s", sanitize(torrentName), hash[:8], name)
		return filepath.Join(cfg.BaseDir, safe)
	default:
		return filepath.Join(cfg.BaseDir, innerPath)
	}
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}
