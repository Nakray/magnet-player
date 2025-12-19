package torrent

import (
	"github.com/anacrolix/torrent"
)

type Engine struct {
	client *torrent.Client
}

func NewEngine(downloadDir string) (*Engine, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = downloadDir
	cfg.NoUpload = false
	cfg.Seed = true
	cfg.DisableIPv6 = false

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Engine{client: client}, nil
}

func (e *Engine) Close() {
	e.client.Close()
}

func (e *Engine) AddMagnet(magnet string) (*torrent.Torrent, error) {
	t, err := e.client.AddMagnet(magnet)
	if err != nil {
		return nil, err
	}
	// Подождать метаданные, чтобы узнать список файлов
	<-t.GotInfo()
	t.DownloadAll()
	t.SetMaxEstablishedConns(80)
	return t, nil
}
