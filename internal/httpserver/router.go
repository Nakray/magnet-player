package httpserver

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"log"
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
	player        *service.PlayerService
	jackettClient *jackett.Client
	mux           *http.ServeMux
}

func NewRouter(p *service.PlayerService, jClient *jackett.Client) http.Handler {
	r := &Router{
		mux:           http.NewServeMux(),
		player:        p,
		jackettClient: jClient,
	}
	r.routes()

	// Middleware с recover от паник
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		r.mux.ServeHTTP(w, req)
	})
}

func (r *Router) routes() {
	// Веб-интерфейс
	r.mux.HandleFunc("/", r.handleIndex)
	r.mux.HandleFunc("/static/", r.handleStatic)

	// API
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

func (r *Router) handleIndex(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	// Читаем index.html из embedded файлов
	data, err := staticFiles.ReadFile("web/static/index.html")
	if err != nil {
		http.Error(w, "Page not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (r *Router) handleStatic(w http.ResponseWriter, req *http.Request) {
	// Извлекаем путь к файлу
	filePath := strings.TrimPrefix(req.URL.Path, "/static/")

	// Читаем файл из embedded
	data, err := staticFiles.ReadFile("web/static/" + filePath)
	if err != nil {
		http.NotFound(w, req)
		return
	}

	// Определяем Content-Type
	contentType := getContentTypeForFile(filePath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

func getContentTypeForFile(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

type addMagnetRequest struct {
	Magnet string `json:"magnet"`
}

func (r *Router) handleAddMagnet(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "method not allowed",
		})
		return
	}

	var body addMagnetRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		log.Printf("handleAddMagnet: decode error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "bad request",
		})
		return
	}

	log.Printf("handleAddMagnet: received magnet: %q", body.Magnet)

	hash, err := r.player.ProcessMagnet(body.Magnet)
	if err != nil {
		log.Printf("handleAddMagnet: ProcessMagnet error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	log.Printf("handleAddMagnet: success, hash: %s", hash)

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

	if r.jackettClient == nil {
		http.Error(w, `{"error": "jackett client not configured"}`, http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()

	results, err := r.jackettClient.Search(ctx, &jackett.SearchQuery{
		Query: query,
	})
	if err != nil {
		log.Printf("Search error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		}); err != nil {
			log.Printf("Failed to encode error response: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(searchResponse{
		Results: results,
		Count:   len(results),
	}); err != nil {
		log.Printf("Failed to encode search response: %v", err)
	}
}

type filesResponse struct {
	Files []*storage.FileMeta `json:"files"`
	Count int                 `json:"count"`
}

func (r *Router) handleFiles(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "method not allowed",
		})
		return
	}

	files := r.player.GetAllFiles()
	if files == nil {
		files = []*storage.FileMeta{}
	}

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

//go:embed web/static/*
var staticFiles embed.FS
