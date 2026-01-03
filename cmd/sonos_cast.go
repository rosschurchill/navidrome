package cmd

import (
	"net/http"

	"github.com/navidrome/navidrome/server/sonos_cast"
)

// Sonos Cast - simple manual instantiation since it doesn't need wire injection
var sonosCastInstance *sonos_cast.SonosCast

func GetSonosCast() *sonos_cast.SonosCast {
	if sonosCastInstance == nil {
		sonosCastInstance = sonos_cast.NewSonosCast()
	}
	return sonosCastInstance
}

func CreateSonosCastRouter() http.Handler {
	ds := CreateDataStore()
	sonosService := GetSonosCast()
	api := sonos_cast.NewAPI(sonosService, ds)
	return api.Router()
}
