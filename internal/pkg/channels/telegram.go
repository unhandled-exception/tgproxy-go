package channels

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
)

var (
	ErrChannelIsFull      = eris.New("The channel is full")
	ErrInvalidMessageType = eris.New("Invalid message type")
)

var (
	TelegramMessageQueueCap = 1000
	TelegramBotAPIURL       = "https://api.telegram.org"
)

var _telegramProviderOptions = []string{
	"timeout",
}

var telegramFatalHTTPStatusCodes = map[int]bool{
	404: true,
	400: true,
}

const (
	httpTimeout     = 1 * time.Second
	httpRetries     = 4
	httpMaxWaitTime = 32 * time.Second
)

var ErrTelegramInvalidTimeout = eris.New("Invalid telegram timeout")

type TelegramMessage struct {
	Text                  string  `json:"text" mapstructure:"text"`
	ParseMode             *string `json:"parse_mode,omitempty" mapstructure:"parse_mode,omitempty"`
	DisableWebPagePreview int     `json:"disable_web_page_preview" mapstructure:"disable_web_page_preview"`
	DisableNotifications  int     `json:"disable_notifications" mapstructure:"disable_notifications"`
	ReplyToMessageID      *int    `json:"reply_to_message_id,omitempty" mapstructure:"reply_to_message_id,omitempty"`
}

func (m *TelegramMessage) String() string {
	res, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(res)
}

func (m *TelegramMessage) Map() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	if err := mapstructure.Decode(m, &res); err != nil {
		return nil, err
	}
	return res, nil
}

type telegramProviderInterface interface {
	SendMessage(*TelegramMessage) error
	HTTPClient() *http.Client
}

type telegramChannel struct {
	chanURL *url.URL

	logger       *zerolog.Logger
	name         string
	queue        chan *TelegramMessage
	provider     telegramProviderInterface
	providerOpts map[string]string

	mu            sync.Mutex
	doneProcesors chan bool
}

func NewTelegramChannel(chanURL *url.URL, logger *zerolog.Logger) (MessageChannelInterface, error) {
	providerOpts := map[string]string{}
	for _, opt := range _telegramProviderOptions {
		if val := chanURL.Query().Get(opt); val != "" {
			providerOpts[opt] = val
		}
	}
	provider, err := NewTelegramChat(TelegramBotAPIURL, chanURL.User.String(), chanURL.Host, providerOpts, logger)
	if err != nil {
		return nil, err
	}

	channel := &telegramChannel{
		chanURL:      chanURL,
		logger:       logger,
		name:         strings.Trim(chanURL.Path, "/"),
		provider:     provider,
		providerOpts: providerOpts,
		queue:        make(chan *TelegramMessage, TelegramMessageQueueCap),
	}

	return channel, nil
}

func (ch *telegramChannel) Stop() error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.doneProcesors != nil {
		close(ch.doneProcesors)
	}
	return nil
}

func (ch *telegramChannel) Start() error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.doneProcesors != nil {
		close(ch.doneProcesors)
	}

	done := make(chan bool, 1)
	go ch.processQueue(done)
	ch.doneProcesors = done

	return nil
}

func (ch *telegramChannel) String() string {
	opts := []string{}
	for k, v := range ch.providerOpts {
		opts = append(opts, k+"="+v)
	}
	fmt.Print(opts)

	var sOpts string
	if len(opts) > 0 {
		sOpts = "?" + strings.Join(opts, "&")
	}

	return fmt.Sprintf(
		"%s://%s:***@%s%s%s",
		ch.chanURL.Scheme, ch.chanURL.User.Username(), ch.chanURL.Hostname(), ch.chanURL.Path, sOpts,
	)
}

func (ch *telegramChannel) Name() string {
	return ch.name
}

func (ch *telegramChannel) Provider() interface{ HTTPClient() *http.Client } {
	return ch.provider
}

func (ch *telegramChannel) MessageContainer() interface{} {
	return &TelegramMessage{}
}

func (ch *telegramChannel) Enqueue(newMessage interface{}) error {
	message, ok := newMessage.(*TelegramMessage)
	if !ok {
		return ErrInvalidMessageType
	}
	select {
	case ch.queue <- message:
	default:
		return ErrChannelIsFull
	}

	ch.logger.Info().Msgf("Enqueue message: %s", message)
	return nil
}

func (ch *telegramChannel) processQueue(done chan bool) {
	for {
		select {
		case <-done:
			return
		case m := <-ch.queue:
			if err := ch.provider.SendMessage(m); err != nil {
				ch.logger.Error().Msgf("Failed to send message: %s", err)
				continue
			}
			ch.logger.Info().Msgf("Message successful sended: %s", m)
		}
	}
}

type telegramChat struct {
	httpClient *resty.Client
	chatID     string
	logger     *zerolog.Logger
}

func NewTelegramChat(apiURL string, botToken string, chatID string, opts map[string]string, logger *zerolog.Logger) (*telegramChat, error) {
	timeout := httpTimeout
	if val, ok := opts["timeout"]; ok {
		timeoutOpt, err := strconv.ParseInt(val, 10, 32) //nolint:gomnd
		if err != nil {
			return nil, err
		}
		timeout = time.Duration(timeoutOpt) * time.Second
	}

	httpClient := resty.New().
		SetHostURL(
			fmt.Sprintf("%s/bot%s", apiURL, botToken),
		).
		SetTimeout(timeout).
		SetRedirectPolicy(resty.NoRedirectPolicy()).
		SetRetryCount(httpRetries).
		SetRetryMaxWaitTime(httpMaxWaitTime).
		AddRetryCondition(
			func(r *resty.Response, err error) bool {
				return telegramFatalHTTPStatusCodes[r.StatusCode()]
			},
		)

	provider := &telegramChat{
		httpClient: httpClient,
		chatID:     chatID,
		logger:     logger,
	}
	return provider, nil
}

func (tc *telegramChat) HTTPClient() *http.Client {
	return tc.httpClient.GetClient()
}

func (tc *telegramChat) SendMessage(message *TelegramMessage) error {
	mm, err := message.Map()
	if err != nil {
		return eris.Wrap(err, "Error on send message")
	}
	mm["chat_id"] = tc.chatID

	body, err := json.Marshal(mm)
	if err != nil {
		return eris.Wrap(err, "Error on send message")
	}

	res, err := tc.httpClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post("/sendMessage")
	if err != nil {
		return eris.Wrap(err, "Error on send message")
	}

	if res.IsError() {
		return eris.New(
			fmt.Sprintf(
				"Failed request to Telegram API: status: %d, body: %s",
				res.StatusCode(), res.Body(),
			),
		)
	}

	return nil
}
