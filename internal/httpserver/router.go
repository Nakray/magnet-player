package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/Nakray/magnet-player/internal/service"
)

type Router struct {
	player *service.PlayerService
	mux    *http.ServeMux
}

func NewRouter(p *service.PlayerService) http.Handler {
	r := &Router{
		mux:    http.NewServeMux(),
		player: p,
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
	// TODO: вытаскивать hash/file index из квери и стримить
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
