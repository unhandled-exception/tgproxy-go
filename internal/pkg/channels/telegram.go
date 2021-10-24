package channels

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
)

const (
	defaultTelegramMessageQueueSize = 10000
	telegramBotAPIURL               = "https://api.telegram.org"
)

var ChannelIsFull = eris.New("The channel is full")

type telegramMessage struct {
	Text                  string  `json:"text" mapstructure:"text"`
	ParseMode             *string `json:"parse_mode,omitempty" mapstructure:"parse_mode,omitempty"`
	DisableWebPagePreview int     `json:"disable_web_page_preview" mapstructure:"disable_web_page_preview"`
	DisableNotifications  int     `json:"disable_notifications" mapstructure:"disable_notifications"`
	ReplyToMessage_id     *int    `json:"reply_to_message_id,omitempty" mapstructure:"reply_to_message_id,omitempty"`
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
	err := mapstructure.Decode(m, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type telegramChannel struct {
	chanURL *url.URL

	logger *zerolog.Logger
	name   string

	queue    chan *telegramMessage
	provider interface {
		SendMessage(*telegramMessage) error
	}
}

func NewTelegramChannel(chanURL *url.URL, logger *zerolog.Logger) (MessageChannelInterface, error) {
	channel := &telegramChannel{
		chanURL: chanURL,
		logger:  logger,
		name:    strings.TrimLeft(chanURL.Path, "/"),

		queue:    make(chan *telegramMessage, defaultTelegramMessageQueueSize),
		provider: NewTelegramChat(telegramBotAPIURL, chanURL.User.String(), chanURL.Host, logger),
	}
	go channel.processQueue()
	return channel, nil
}

func (ch *telegramChannel) String() string {
	return fmt.Sprintf(
		"%s://%s:***@%s/%s?%s",
		ch.chanURL.Scheme, ch.chanURL.User.Username(), ch.chanURL.Hostname(), ch.chanURL.Path, ch.chanURL.RawQuery,
	)
}

func (ch *telegramChannel) Name() string {
	return ch.name
}

func (ch *telegramChannel) MessageContainer() interface{} {
	return &telegramMessage{}
}

func (ch *telegramChannel) Enqueue(newMessage interface{}) error {
	message := newMessage.(*telegramMessage)
	select {
	case ch.queue <- message:
	default:
		return ChannelIsFull
	}

	ch.logger.Info().Msgf("Enqueue message: %s", message)
	return nil
}

func (ch *telegramChannel) processQueue() {
	for m := range ch.queue {
		err := ch.provider.SendMessage(m)
		if err != nil {
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

func NewTelegramChat(apiURL string, botToken string, chatID string, logger *zerolog.Logger) *telegramChat {
	client := resty.New()
	client.
		SetHostURL(
			fmt.Sprintf("%s/bot%s", apiURL, botToken),
		).
		SetTimeout(5 * time.Second).
		SetRedirectPolicy(resty.NoRedirectPolicy()).
		SetRetryCount(4).
		SetRetryMaxWaitTime(32 * time.Second).
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
	return provider
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
	tc.logger.Info().Msg(string(body))

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
