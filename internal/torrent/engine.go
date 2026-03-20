package torrent

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
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

// AddTorrentFromURL скачивает .torrent файл по URL и добавляет его
func (e *Engine) AddTorrentFromURL(url string) (*torrent.Torrent, error) {
	// Создаём клиент, который не следует редиректам автоматически
	// чтобы мы могли перехватить magnet-ссылку
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Возвращаем ошибку, чтобы остановить редиректы
			// и проверить последний ответ
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("AddTorrentFromURL: status=%d, location=%s", resp.StatusCode, resp.Header.Get("Location"))

	// Проверяем, есть ли redirect на magnet-ссылку
	location := resp.Header.Get("Location")
	if location != "" && len(location) > 7 && location[:7] == "magnet:" {
		log.Printf("AddTorrentFromURL: redirect to magnet link, using it directly")
		return e.client.AddMagnet(location)
	}

	// Если нет redirect'а, читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Проверяем, не вернул ли Jackett magnet-ссылку в теле ответа
	if bytes.HasPrefix(body, []byte("magnet:?")) {
		magnetLink := string(bytes.TrimSpace(body))
		log.Printf("AddTorrentFromURL: Jackett returned magnet link in body, using it directly")
		return e.client.AddMagnet(magnetLink)
	}

	// Проверяем Content-Type для .torrent файла
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/x-bittorrent" && contentType != "application/octet-stream" {
		return nil, fmt.Errorf("unexpected content type: %s, body: %s", contentType, string(body))
	}

	mi, err := metainfo.Load(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return e.client.AddTorrent(mi)
}
