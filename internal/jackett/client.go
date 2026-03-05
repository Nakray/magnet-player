package jackett

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SearchResult — результат поиска из Jackett API
type SearchResult struct {
	Title       string  `json:"title"`
	Size        int64   `json:"size"`
	Seeders     int     `json:"seeders"`
	Peers       int     `json:"peers"`
	Grabs       int     `json:"grabs"`
	Category    string  `json:"category"`
	PublishDate string  `json:"publish_date"`
	MagnetLink  string  `json:"magnet_link"`
	Indexer     string  `json:"indexer"`
}

// SearchQuery — параметры поиска
type SearchQuery struct {
	Query    string
	Category string // опционально: "audio", "video", etc.
	MinSize  int64  // опционально: минимальный размер в байтах
}

// Client — HTTP-клиент для работы с Jackett API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient создаёт новый клиент для Jackett API
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Search выполняет поиск торрентов через Jackett API
func (c *Client) Search(ctx context.Context, q *SearchQuery) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("apikey", c.apiKey)
	params.Set("q", q.Query)
	params.Set("t", "search")
	params.Set("o", "json")

	if q.Category != "" {
		params.Set("cat", q.Category)
	}

	reqURL := fmt.Sprintf("%s/api/v2.0/indexers/all/results?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jackett API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var rawResults []struct {
		Title       string `json:"Title"`
		Size        int64  `json:"Size"`
		Seeders     int    `json:"Seeders"`
		Peers       int    `json:"Peers"`
		Grabs       int    `json:"Grabs"`
		Category    string `json:"Category"`
		PublishDate string `json:"PublishDate"`
		MagnetURI   string `json:"MagnetUri"`
		Indexer     string `json:"Indexer"`
	}

	if err := json.Unmarshal(body, &rawResults); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	results := make([]SearchResult, 0, len(rawResults))
	for _, r := range rawResults {
		// Фильтрация по минимальному размеру
		if q.MinSize > 0 && r.Size < q.MinSize {
			continue
		}

		results = append(results, SearchResult{
			Title:       r.Title,
			Size:        r.Size,
			Seeders:     r.Seeders,
			Peers:       r.Peers,
			Grabs:       r.Grabs,
			Category:    r.Category,
			PublishDate: r.PublishDate,
			MagnetLink:  r.MagnetURI,
			Indexer:     r.Indexer,
		})
	}

	return results, nil
}

// TestConnection проверяет доступность Jackett API
func (c *Client) TestConnection(ctx context.Context) error {
	params := url.Values{}
	params.Set("apikey", c.apiKey)
	params.Set("t", "search")
	params.Set("q", "test")
	params.Set("o", "json")

	reqURL := fmt.Sprintf("%s/api/v2.0/indexers/all/results?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jackett API returned status %d", resp.StatusCode)
	}

	return nil
}
