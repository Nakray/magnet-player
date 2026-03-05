package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nakray/magnet-player/internal/jackett"
	"github.com/Nakray/magnet-player/internal/service"
	"github.com/Nakray/magnet-player/internal/storage"
)

type Router struct {
	player       *service.PlayerService
	jackettClient *jackett.Client
	mux          *http.ServeMux
}

func NewRouter(p *service.PlayerService, jClient *jackett.Client) http.Handler {
	r := &Router{
		mux:           http.NewServeMux(),
		player:        p,
		jackettClient: jClient,
	}
	r.routes()
	return r.mux
}

func (r *Router) routes() {
	r.mux.HandleFunc("/health", r.handleHealth)
	r.mux.HandleFunc("/api/add-magnet", r.handleAddMagnet)
	r.mux.HandleFunc("/api/search", r.handleSearch)
	r.mux.HandleFunc("/api/stream", r.handleStream)
	r.mux.HandleFunc("/api/files", r.handleFiles)
	r.mux.HandleFunc("/api/files/", r.handleFileDelete)
}

func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

type addMagnetRequest struct {
	Magnet string `json:"magnet"`
}

func (r *Router) handleAddMagnet(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body addMagnetRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	hash, err := r.player.ProcessMagnet(body.Magnet)
	if err != nil {
		http.Error(w, "failed to add magnet", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "added",
		"hash":   hash,
	})
}

func (r *Router) handleStream(w http.ResponseWriter, req *http.Request) {
	hash := req.URL.Query().Get("hash")
	if hash == "" {
		http.Error(w, `{"error": "hash parameter is required"}`, http.StatusBadRequest)
		return
	}

	meta := r.player.GetFileMeta(hash)
	if meta == nil {
		http.Error(w, `{"error": "file not found"}`, http.StatusNotFound)
		return
	}

	// Открываем файл для чтения
	file, err := os.Open(meta.Path)
	if err != nil {
		http.Error(w, `{"error": "failed to open file"}`, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Обработка Range заголовка
	rangeHeader := req.Header.Get("Range")
	if rangeHeader != "" {
		r.handleRangeRequest(w, req, hash, rangeHeader, file, meta.Size)
		return
	}

	// Отправка всего файла
	w.Header().Set("Content-Type", getContentType(hash))
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", meta.LastAccess.Format(http.TimeFormat))

	http.ServeContent(w, req, filepath.Base(hash), meta.LastAccess, file)
}

func (r *Router) handleRangeRequest(w http.ResponseWriter, req *http.Request, hash, rangeHeader string, file *os.File, fileSize int64) {
	// Парсинг Range: bytes=start-end
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeHeader, "-")
	if len(parts) != 2 {
		http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid range start", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	var end int64 = fileSize - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			http.Error(w, "Invalid range end", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= fileSize {
			end = fileSize - 1
		}
	}

	if start > end {
		http.Error(w, "Invalid range: start > end", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1

	w.Header().Set("Content-Type", getContentType(hash))
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(fileSize, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusPartialContent)

	if _, err := file.Seek(start, 0); err != nil {
		return
	}

	io.CopyN(w, file, contentLength)
}

func getContentType(hash string) string {
	ext := filepath.Ext(hash)
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".m4a":
		return "audio/mp4"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "application/octet-stream"
	}
}

type searchResponse struct {
	Results []jackett.SearchResult `json:"results"`
	Count   int                    `json:"count"`
}

func (r *Router) handleSearch(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := req.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error": "query parameter 'q' is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()

	results, err := r.jackettClient.Search(ctx, &jackett.SearchQuery{
		Query: query,
	})
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(searchResponse{
		Results: results,
		Count:   len(results),
	})
}

type filesResponse struct {
	Files []*storage.FileMeta `json:"files"`
	Count int                 `json:"count"`
}

func (r *Router) handleFiles(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files := r.player.GetAllFiles()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(filesResponse{
		Files: files,
		Count: len(files),
	})
}

func (r *Router) handleFileDelete(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем hash из пути /api/files/:hash
	hash := strings.TrimPrefix(req.URL.Path, "/api/files/")
	if hash == "" || hash == "/api/files" {
		http.Error(w, `{"error": "hash is required"}`, http.StatusBadRequest)
		return
	}

	if err := r.player.RemoveFile(hash); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"hash":   hash,
	})
}
