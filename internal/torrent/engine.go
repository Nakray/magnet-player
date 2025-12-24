package torrent

import (
	"github.com/anacrolix/torrent"
)

type Engine struct {
	client *torrent.Client
}

func NewEngine() (*Engine, error) {
	tCfg := torrent.NewDefaultClientConfig()
	tCfg.NoUpload = false
	tCfg.Seed = true
	tCfg.DisableIPv6 = false

	client, err := torrent.NewClient(tCfg)
	if err != nil {
		return nil, err
	}

	return &Engine{client: client}, nil
}

func (e *Engine) Close() {
	e.client.Close()
}

func (e *Engine) AddMagnet(magnet string) (*torrent.Torrent, error) {
	return e.client.AddMagnet(magnet)
}
