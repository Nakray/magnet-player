package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Server       ServerConfig       `json:"server"`
	Storage      StorageConfig      `json:"storage"`
	Torrent      TorrentConfig      `json:"torrent"`
	Transcoding  TranscodingConfig  `json:"transcoding"`
	Logging      LoggingConfig      `json:"logging"`
}

type ServerConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	ReadTimeoutSec  int    `json:"read_timeout_sec"`
	WriteTimeoutSec int    `json:"write_timeout_sec"`
}

func (s *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type StorageConfig struct {
	BaseDir         string `json:"base_dir"`
	DbPath          string `json:"db_path"`
	MaxSizeGB       int64  `json:"max_size_gb"`
	EvictionPolicy  string `json:"eviction_policy"`
	FolderMode      string `json:"folder_mode"`
}

type TorrentConfig struct {
	MaxActiveTorrents         int  `json:"max_active_torrents"`
	MaxConnectionsPerTorrent  int  `json:"max_connections_per_torrent"`
	MaxEstablishedConns       int  `json:"max_established_conns"`
	UploadEnabled             bool `json:"upload_enabled"`
	SeedEnabled               bool `json:"seed_enabled"`
	IPv6Disabled              bool `json:"ipv6_disabled"`
}

type TranscodingConfig struct {
	Enabled     bool   `json:"enabled"`
	FFmpegPath  string `json:"ffmpeg_path"`
	DefaultFormat string `json:"default_format"`
	Bitrate     string `json:"bitrate"`
}

type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Валидация значений по умолчанию
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeoutSec == 0 {
		cfg.Server.ReadTimeoutSec = 30
	}
	if cfg.Server.WriteTimeoutSec == 0 {
		cfg.Server.WriteTimeoutSec = 30
	}

	if cfg.Storage.BaseDir == "" {
		cfg.Storage.BaseDir = "./data"
	}
	if cfg.Storage.DbPath == "" {
		cfg.Storage.DbPath = "./data/meta.db"
	}
	if cfg.Storage.MaxSizeGB == 0 {
		cfg.Storage.MaxSizeGB = 10
	}
	if cfg.Storage.EvictionPolicy == "" {
		cfg.Storage.EvictionPolicy = "lru"
	}
	if cfg.Storage.FolderMode == "" {
		cfg.Storage.FolderMode = "structured"
	}

	if cfg.Torrent.MaxActiveTorrents == 0 {
		cfg.Torrent.MaxActiveTorrents = 5
	}
	if cfg.Torrent.MaxConnectionsPerTorrent == 0 {
		cfg.Torrent.MaxConnectionsPerTorrent = 40
	}
	if cfg.Torrent.MaxEstablishedConns == 0 {
		cfg.Torrent.MaxEstablishedConns = 80
	}

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}

	return &cfg, nil
}
