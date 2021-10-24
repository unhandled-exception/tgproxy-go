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

	"internal/pkg/channels"
)

var ChannelNotFound = eris.New("Channel not found")

type httpAPI struct {
	router      *chi.Mux
	logger      zerolog.Logger
	channelsMap map[string]channels.MessageChannelInterface
}

func NewHTTPAPI(messagesChannels []channels.MessageChannelInterface, logger zerolog.Logger) *httpAPI {
	router := chi.NewRouter()

	channelsMap := map[string]channels.MessageChannelInterface{}
	for _, ch := range messagesChannels {
		channelsMap[ch.Name()] = ch
	}

	api := &httpAPI{
		router:      router,
		logger:      logger,
		channelsMap: channelsMap,
	}

	router.Use(httplog.RequestLogger(logger))
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(5 * time.Second))

	router.Get("/ping.html", api.onPing)
	router.Get("/", api.onIndex)
	router.Post("/{channelName}", api.onSend)

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
		Status   string            `json:"status"`
		Channels map[string]string `json:"channels"`
	}{
		Status:   "success",
		Channels: map[string]string{},
	}
	for k, v := range api.channelsMap {
		response.Channels[k] = fmt.Sprint(v)
	}

	render.JSON(w, r, response)
}

func (api *httpAPI) getChannel(channelName string) (channels.MessageChannelInterface, error) {
	ch, ok := api.channelsMap[channelName]
	if !ok {
		return nil, eris.Wrap(ChannelNotFound, channelName)
	}
	return ch, nil
}

func (api *httpAPI) renderError(w http.ResponseWriter, r *http.Request, err error, httpStatusCode int) {
	render.Status(r, httpStatusCode)
	render.JSON(w, r, map[string]string{
		"status":  "error",
		"message": err.Error(),
	})
}

func (api *httpAPI) onSend(w http.ResponseWriter, r *http.Request) {
	ch, err := api.getChannel(chi.URLParam(r, "channelName"))
	if err != nil {
		api.renderError(w, r, err, 404)
		return
	}

	message := ch.MessageContainer()
	err = render.DecodeJSON(r.Body, message)
	if err != nil {
		api.renderError(w, r, err, 400)
		return
	}

	err = ch.Enqueue(message)
	if err != nil {
		api.renderError(w, r, err, 503)
		return
	}

	render.JSON(w, r, map[string]string{
		"status": "success",
	})

}
