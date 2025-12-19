package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/Nakray/magnet-player/internal/storage"
	"github.com/Nakray/magnet-player/internal/torrent"
)

type Router struct {
	engine *torrent.Engine
	cache  *storage.CacheManager
	metaDB *storage.MetadataDB
	mux    *http.ServeMux
}

func NewRouter(engine *torrent.Engine, cache *storage.CacheManager, metaDB *storage.MetadataDB) http.Handler {
	r := &Router{
		engine: engine,
		cache:  cache,
		metaDB: metaDB,
		mux:    http.NewServeMux(),
	}
	r.routes()
	return r.mux
}

func (r *Router) routes() {
	r.mux.HandleFunc("/health", r.handleHealth)
	r.mux.HandleFunc("/api/add-magnet", r.handleAddMagnet)
	r.mux.HandleFunc("/api/stream", r.handleStream)
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

	t, err := r.engine.AddMagnet(body.Magnet)
	if err != nil {
		http.Error(w, "failed to add magnet", http.StatusInternalServerError)
		return
	}

	_ = t // TODO: сохранить в metaDB

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "added",
	})
}

func (r *Router) handleStream(w http.ResponseWriter, req *http.Request) {
	// TODO: вытаскивать hash/file index из квери и стримить
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
