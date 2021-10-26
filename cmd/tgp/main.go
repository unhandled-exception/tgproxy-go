package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/unhandled-exception/tgproxy-go/internal/pkg/channels"
	"github.com/unhandled-exception/tgproxy-go/internal/pkg/httpapi"

	"github.com/go-chi/httplog"
)

const programHelp = `
A simple Telegram proxy

Start server:
tgp telegram://bot:tok1@123/chat_1 telegram://bot:tok1@123/chat_2?timeout=3

API:
Get ping-status — GET http://localhost:5000/ping
Get channels list — GET http://localhost:5000/
Send messge POST http://localhost:5000/chat_1 json_body("text":"Message", "parse_mode": ...)
Get channel statistics GET http://localhost:5000/chat_1

Run program:
tgp [-H localhost] [-P port] channels_urls

Required arguments:
channels_urls — List of channels uri. Formatting: telegram://bot:token@chat_id/channel_name?timeout=value

Optional arguments:
`

const InvalidArgumentExitCode = 2

func main() {
	var host string
	var port string
	flag.StringVar(&host, "H", "localhost", "Server host")
	flag.StringVar(&port, "P", "5000", "Server port")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, strings.TrimSpace(programHelp))
		flag.PrintDefaults()
		os.Exit(1)
	}

	flag.Parse()

	channelsURLS := flag.Args()
	if len(channelsURLS) == 0 {
		printCommandLineErrorAndExit("Requires list of channels", InvalidArgumentExitCode)
	}

	logger := httplog.NewLogger("tgp-api")

	messageChannels, err := channels.BuildChannelsFromURLS(channelsURLS, &logger)
	if err != nil {
		printCommandLineErrorAndExit(err.Error(), InvalidArgumentExitCode)
	}

	logger.Info().Msg(fmt.Sprint(messageChannels))

	api := httpapi.NewHTTPAPI(messageChannels, &logger)
	if err := api.StartAllChannels(); err != nil {
		logger.Fatal().Err(err)
	}

	hostAndPort := fmt.Sprintf("%s:%s", host, port)
	logger.Info().Msgf("Start http server on %s", hostAndPort)
	err = http.ListenAndServe(
		hostAndPort,
		api,
	)
	if err != nil {
		logger.Fatal().Err(err)
	}
}

func printCommandLineErrorAndExit(msg string, exitCode int) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintf(os.Stderr, "---------------------\n\n")
	flag.Usage()
	os.Exit(exitCode)
}
