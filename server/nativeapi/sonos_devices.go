package nativeapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

// SonosDeviceResponse represents a Sonos device for the API
type SonosDeviceResponse struct {
	ID          string `json:"id"`
	HouseholdID string `json:"householdId"`
	DeviceName  string `json:"deviceName"`
	LastSeenAt  string `json:"lastSeenAt"`
	CreatedAt   string `json:"createdAt"`
}

func (api *Router) addSonosDevicesRoute(r chi.Router) {
	r.Route("/sonos/devices", func(r chi.Router) {
		r.Get("/", getSonosDevices(api.ds))
		r.Delete("/{id}", revokeSonosDevice(api.ds))
	})
}

// getSonosDevices returns all Sonos devices linked to the current user
func getSonosDevices(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, ok := request.UserFrom(ctx)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		devices, err := ds.SonosDeviceToken(ctx).GetByUserID(user.ID)
		if err != nil {
			log.Error(ctx, "Error getting Sonos devices", "user", user.UserName, err)
			http.Error(w, "Error retrieving devices", http.StatusInternalServerError)
			return
		}

		response := make([]SonosDeviceResponse, len(devices))
		for i, device := range devices {
			response[i] = SonosDeviceResponse{
				ID:          device.ID,
				HouseholdID: device.HouseholdID,
				DeviceName:  device.DeviceName,
				LastSeenAt:  device.LastSeenAt.Format("2006-01-02T15:04:05Z"),
				CreatedAt:   device.CreatedAt.Format("2006-01-02T15:04:05Z"),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Error encoding response", err)
		}
	}
}

// revokeSonosDevice removes a Sonos device token
func revokeSonosDevice(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user, ok := request.UserFrom(ctx)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		deviceID := chi.URLParam(r, "id")
		if deviceID == "" {
			http.Error(w, "Device ID required", http.StatusBadRequest)
			return
		}

		// Get the device to verify ownership
		device, err := ds.SonosDeviceToken(ctx).Get(deviceID)
		if err != nil {
			if err == model.ErrNotFound {
				http.Error(w, "Device not found", http.StatusNotFound)
				return
			}
			log.Error(ctx, "Error getting Sonos device", "id", deviceID, err)
			http.Error(w, "Error retrieving device", http.StatusInternalServerError)
			return
		}

		// Check ownership (only the owner or admin can revoke)
		if device.UserID != user.ID && !user.IsAdmin {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Delete the device token
		if err := ds.SonosDeviceToken(ctx).Delete(deviceID); err != nil {
			log.Error(ctx, "Error revoking Sonos device", "id", deviceID, err)
			http.Error(w, "Error revoking device", http.StatusInternalServerError)
			return
		}

		log.Info(ctx, "Sonos device revoked", "id", deviceID, "user", user.UserName)
		w.WriteHeader(http.StatusNoContent)
	}
}
