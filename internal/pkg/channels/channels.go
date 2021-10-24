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

type MessageChannelInterface interface {
	fmt.Stringer
	Name() string
	MessageContainer() interface{}
	Enqueue(interface{}) error
}

var channelsTypes = map[string]func(chanURL *url.URL, logger *zerolog.Logger) (MessageChannelInterface, error){}

func BuildChannelsFromURLS(urls []string, logger *zerolog.Logger) ([]MessageChannelInterface, error) {
	result := []MessageChannelInterface{}
	for _, chanURL := range urls {
		u, err := url.Parse(chanURL)
		if err != nil {
			return nil, err
		}
		ch, err := BuildChannel(u, logger)
		if err != nil {
			return nil, err
		}
		result = append(result, ch)
	}
	return result, nil
}

func BuildChannel(chanURL *url.URL, logger *zerolog.Logger) (MessageChannelInterface, error) {
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
