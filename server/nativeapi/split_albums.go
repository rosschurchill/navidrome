package nativeapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// Split albums endpoints (admin only)
func (api *Router) addSplitAlbumsRoute(r chi.Router) {
	r.Route("/splitAlbums", func(r chi.Router) {
		r.Get("/", getSplitAlbums(api.ds))
		r.Post("/merge", mergeAlbums(api.ds))
	})
}

// getSplitAlbums returns albums that have been incorrectly split into multiple entries
func getSplitAlbums(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		albumRepo := ds.Album(ctx)
		splitAlbums, err := albumRepo.GetSplitAlbums()
		if err != nil {
			log.Error(ctx, "Error getting split albums", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(splitAlbums); err != nil {
			log.Error(ctx, "Error encoding split albums response", err)
		}
	}
}

// mergeAlbums merges multiple album entries under a single album artist
func mergeAlbums(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var request struct {
			AlbumIDs          []string `json:"albumIds"`
			TargetAlbumArtist string   `json:"targetAlbumArtist"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			log.Error(ctx, "Error decoding merge albums request", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(request.AlbumIDs) < 2 {
			http.Error(w, "At least 2 album IDs required", http.StatusBadRequest)
			return
		}

		if request.TargetAlbumArtist == "" {
			http.Error(w, "Target album artist is required", http.StatusBadRequest)
			return
		}

		albumRepo := ds.Album(ctx)
		if err := albumRepo.MergeAlbums(request.AlbumIDs, request.TargetAlbumArtist); err != nil {
			log.Error(ctx, "Error merging albums", "albumIds", request.AlbumIDs, "targetArtist", request.TargetAlbumArtist, err)
			http.Error(w, "Failed to merge albums", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	}
}
