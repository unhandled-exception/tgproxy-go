package httpapi

import (
	"fmt"
	"internal/pkg/channels"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/httplog"
	"github.com/go-chi/render"
	"github.com/rs/zerolog"
)

type httpAPI struct {
	router   *chi.Mux
	logger   zerolog.Logger
	channels []channels.Channel
}

func NewHTTPAPI(channels []channels.Channel, logger zerolog.Logger) *httpAPI {
	router := chi.NewRouter()
	api := &httpAPI{
		router:   router,
		logger:   logger,
		channels: channels,
	}

	router.Use(httplog.RequestLogger(logger))
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(5 * time.Second))

	router.Get("/ping.html", api.onPing)
	router.Get("/", api.onIndex)

	return api
}

func (api *httpAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.router.ServeHTTP(w, r)
}

func (api *httpAPI) onPing(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r,
		map[string]string{
			"status": "success",
		},
	)
}

func (api *httpAPI) onIndex(w http.ResponseWriter, r *http.Request) {
	response := struct {
		Status   string   `json:"status"`
		Channels []string `json:"channels"`
	}{
		Status: "success",
	}
	for _, ch := range api.channels {
		response.Channels = append(response.Channels, fmt.Sprint(ch))
	}

	render.JSON(w, r, response)
}
