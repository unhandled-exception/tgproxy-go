package channels

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
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
	_telegramMessageQueueSize = 10000
	_telegramBotAPIURL        = "https://api.telegram.org"
)

var _telegramProviderOptions = []string{
	"timeout",
}

type telegramMessage struct {
	Text                  string  `json:"text" mapstructure:"text"`
	ParseMode             *string `json:"parse_mode,omitempty" mapstructure:"parse_mode,omitempty"`
	DisableWebPagePreview int     `json:"disable_web_page_preview" mapstructure:"disable_web_page_preview"`
	DisableNotifications  int     `json:"disable_notifications" mapstructure:"disable_notifications"`
	ReplyToMessageID      *int    `json:"reply_to_message_id,omitempty" mapstructure:"reply_to_message_id,omitempty"`
}

func (m *telegramMessage) String() string {
	res, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(res)
}

func (m *telegramMessage) Map() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	if err := mapstructure.Decode(m, &res); err != nil {
		return nil, err
	}
	return res, nil
}

type telegramChannel struct {
	chanURL *url.URL

	logger   *zerolog.Logger
	name     string
	queue    chan *telegramMessage
	provider interface {
		SendMessage(*telegramMessage) error
	}
	providerOpts map[string]string
}

func NewTelegramChannel(chanURL *url.URL, logger *zerolog.Logger) (MessageChannelInterface, error) {
	providerOpts := map[string]string{}
	for _, opt := range _telegramProviderOptions {
		if val := chanURL.Query().Get(opt); val != "" {
			providerOpts[opt] = val
		}
	}
	provider, err := NewTelegramChat(_telegramBotAPIURL, chanURL.User.String(), chanURL.Host, providerOpts, logger)
	if err != nil {
		return nil, err
	}

	channel := &telegramChannel{
		chanURL:      chanURL,
		logger:       logger,
		name:         strings.Trim(chanURL.Path, "/"),
		queue:        make(chan *telegramMessage, _telegramMessageQueueSize),
		provider:     provider,
		providerOpts: providerOpts,
	}
	go channel.processQueue()

	return channel, nil
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

func (ch *telegramChannel) MessageContainer() interface{} {
	return &telegramMessage{}
}

func (ch *telegramChannel) Enqueue(newMessage interface{}) error {
	message, ok := newMessage.(*telegramMessage)
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

func (ch *telegramChannel) processQueue() {
	for m := range ch.queue {
		if err := ch.provider.SendMessage(m); err != nil {
			ch.logger.Error().Msgf("Failed to send message: %s", err)
			continue
		}
		ch.logger.Info().Msgf("Message successful sended: %s", m)
	}
}

type telegramChat struct {
	client *resty.Client
	chatID string
	logger *zerolog.Logger
}

var telegramFatalHTTPStatusCodes = map[int]bool{
	404: true,
	400: true,
}

const (
	httpTimeout     = 5 * time.Second
	httpRetries     = 4
	httpMaxWaitTime = 32 * time.Second
)

var ErrTelegramInvalidTimeout = eris.New("Invalid telegram timeout")

func NewTelegramChat(apiURL string, botToken string, chatID string, opts map[string]string, logger *zerolog.Logger) (*telegramChat, error) {
	timeout := httpTimeout
	if val, ok := opts["timeout"]; ok {
		timeoutOpt, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return nil, err
		}
		timeout = time.Duration(timeoutOpt) * time.Second
	}

	client := resty.New().
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
		client: client,
		chatID: chatID,
		logger: logger,
	}
	return provider, nil
}

func (tc *telegramChat) SendMessage(message *telegramMessage) error {
	mm, err := message.Map()
	if err != nil {
		return eris.Wrap(err, "Error on send message")
	}
	mm["chat_id"] = tc.chatID

	body, err := json.Marshal(mm)
	if err != nil {
		return eris.Wrap(err, "Error on send message")
	}

	res, err := tc.client.R().
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
