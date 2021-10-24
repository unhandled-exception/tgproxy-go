package channels

import (
	"fmt"
	"net/url"

	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
)

var (
	UnknownChannelType = eris.New("Unknown channel type")
	CreateChannelErorr = eris.New("Error on create channel")
)

type Channel interface {
	fmt.Stringer
}

type telegramChannel struct {
	chanURL *url.URL
	logger  *zerolog.Logger
}

func NewTelegramChannel(chanURL *url.URL, logger *zerolog.Logger) (Channel, error) {
	channel := &telegramChannel{
		chanURL: chanURL,
	}
	return channel, nil
}

func (ch *telegramChannel) String() string {
	return fmt.Sprintf(
		"%s://%s:***@%s/%s?%s",
		ch.chanURL.Scheme, ch.chanURL.User.Username(), ch.chanURL.Hostname(), ch.chanURL.Path, ch.chanURL.RawQuery,
	)
}

var channelsTypes = map[string]func(chanURL *url.URL, logger *zerolog.Logger) (Channel, error){}

func BuildChannel(chanURL *url.URL, logger *zerolog.Logger) (Channel, error) {
	channelConstructor, ok := channelsTypes[chanURL.Scheme]
	if !ok {
		return nil, eris.Wrapf(UnknownChannelType, "scheme: %s", chanURL.Scheme)
	}
	channel, err := channelConstructor(chanURL, logger)
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func init() {
	channelsTypes["telegram"] = NewTelegramChannel
}
