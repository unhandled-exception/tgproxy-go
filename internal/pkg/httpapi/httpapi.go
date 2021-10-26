package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/httplog"
	"github.com/go-chi/render"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"

	"github.com/unhandled-exception/tgproxy-go/internal/pkg/channels"
)

var ErrChannelNotFound = eris.New("Channel not found")

type HTTPAPI struct {
	router      *chi.Mux
	logger      *zerolog.Logger
	channelsMap map[string]channels.MessageChannelInterface
}

func NewHTTPAPI(messagesChannels []channels.MessageChannelInterface, logger *zerolog.Logger) *HTTPAPI {
	router := chi.NewRouter()

	channelsMap := map[string]channels.MessageChannelInterface{}
	for _, ch := range messagesChannels {
		channelsMap[ch.Name()] = ch
	}

	api := &HTTPAPI{
		router:      router,
		logger:      logger,
		channelsMap: channelsMap,
	}

	router.Use(httplog.RequestLogger(*logger))
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(5 * time.Second))
	router.Use(middleware.StripSlashes)

	router.Get("/ping", api.onPing)
	router.Get("/", api.onIndex)
	router.Post("/{channelName}", api.onSend)

	return api
}

func (api *HTTPAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	api.router.ServeHTTP(w, r)
}

func (api *HTTPAPI) GetChannel(name string) (channels.MessageChannelInterface, error) {
	ch, ok := api.channelsMap[name]
	if !ok {
		return nil, eris.Wrap(ErrChannelNotFound, name)
	}
	return ch, nil
}

func (api *HTTPAPI) StartAllChannels() error {
	for _, v := range api.channelsMap {
		if err := v.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (api *HTTPAPI) StopChannel(name string) error {
	ch, err := api.GetChannel(name)
	if err != nil {
		return err
	}

	if err := ch.Stop(); err != nil {
		return nil
	}
	return nil
}

func (api *HTTPAPI) onPing(w http.ResponseWriter, r *http.Request) {
	api.renderSuccess(w, r, nil, http.StatusOK)
}

func (api *HTTPAPI) onIndex(w http.ResponseWriter, r *http.Request) {
	channels := map[string]string{}
	for k, v := range api.channelsMap {
		channels[k] = fmt.Sprint(v)
	}

	api.renderSuccess(w, r, map[string]interface{}{
		"channels": channels,
	}, http.StatusOK)
}

func (api *HTTPAPI) onSend(w http.ResponseWriter, r *http.Request) {
	ch, err := api.GetChannel(chi.URLParam(r, "channelName"))
	if err != nil {
		api.renderError(w, r, err, http.StatusNotFound)
		return
	}

	message := ch.MessageContainer()
	if err := render.DecodeJSON(r.Body, message); err != nil {
		api.renderError(w, r, err, http.StatusBadRequest)
		return
	}

	if err := ch.Enqueue(message); err != nil {
		api.renderError(w, r, err, http.StatusServiceUnavailable)
		return
	}

	api.renderSuccess(w, r, nil, http.StatusCreated)
}

func (api *HTTPAPI) renderError(w http.ResponseWriter, r *http.Request, err error, httpStatusCode int) {
	render.Status(r, httpStatusCode)
	render.JSON(w, r, map[string]string{
		"status":  "error",
		"message": err.Error(),
	})
}

func (api *HTTPAPI) renderSuccess(w http.ResponseWriter, r *http.Request, data map[string]interface{}, httpStatusCode int) {
	responseData := map[string]interface{}{
		"status": "success",
	}
	for k, v := range data {
		responseData[k] = v
	}

	render.Status(r, httpStatusCode)
	render.JSON(w, r, responseData)
}
