//go:build !race

package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"github.com/unhandled-exception/tgproxy-go/internal/pkg/channels"
	"github.com/unhandled-exception/tgproxy-go/internal/pkg/httpapi"
	"golang.org/x/net/context"
)

var (
	_testChannels = []string{
		"telegram://bot:token@chat_1/main/?timeout=100",
		"telegram://bot2:token2@chat_2/second",
	}
	_testTelegamAPISendMessageMethod = "https://api.telegram.org/botbot:token/sendMessage"
	_testAPIURL                      = "http://api"
	_testContentTypeHeader           = "Content-Type"
	_testApplicationJSONCT           = "application/json"
)

type httpapiTestSuite struct {
	suite.Suite

	sut *httpapi.HTTPAPI
}

func TestHttpAPI(t *testing.T) {
	suite.Run(t, new(httpapiTestSuite))
}

func (ts *httpapiTestSuite) SetupTest() {
	logger := zerolog.New(io.Discard)
	// logger := zerolog.New(zerolog.NewConsoleWriter())

	chs, err := channels.BuildChannelsFromURLS(_testChannels, &logger)
	if err != nil {
		ts.FailNow("Failed to init test app: %e", err)
	}

	for _, ch := range chs {
		httpmock.ActivateNonDefault(ch.Provider().HTTPClient())
	}

	ts.sut = httpapi.NewHTTPAPI(chs, &logger)
	err = ts.sut.StartAllChannels()
	if err != nil {
		ts.FailNow("Failed to init test app: %e", err)
	}

	httpmock.Reset()
}

func (ts *httpapiTestSuite) TearDownTest() {
	ts.sut = nil
}

func (ts *httpapiTestSuite) sutRequest(method string, url string, requestBody map[string]interface{}) (*http.Response, string) {
	rb, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(method, url, bytes.NewReader(rb))
	w := httptest.NewRecorder()

	ts.sut.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	return resp, string(body)
}

func (ts *httpapiTestSuite) TestPing() {
	resp, body := ts.sutRequest(http.MethodGet, "http://api/ping", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)
	ts.Equal(resp.Header.Get(_testContentTypeHeader), _testApplicationJSONCT)
	ts.JSONEq(
		`{"status": "success"}`,
		body,
	)
}

func (ts *httpapiTestSuite) TestGetIndex() {
	resp, body := ts.sutRequest(http.MethodGet, "http://api/", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)
	ts.Equal(resp.Header.Get(_testContentTypeHeader), _testApplicationJSONCT)
	ts.JSONEq(
		`{
			"status": "success",
			"channels": {
				"main":"telegram://bot:***@chat_1/main/?timeout=100",
				"second":"telegram://bot2:***@chat_2/second"
			}
		}`,
		body,
	)
}

type mockedRequest struct {
	URL    string
	Method string
	Header http.Header
	Body   string
}

func newMockedRequest(req *http.Request) (*mockedRequest, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()

	mr := &mockedRequest{
		URL:    req.URL.String(),
		Method: req.Method,
		Body:   string(body),
		Header: req.Header,
	}
	return mr, nil
}

func (ts *httpapiTestSuite) TestSendMessageOk() {
	var mr *mockedRequest
	httpmock.RegisterResponder(http.MethodPost, _testTelegamAPISendMessageMethod,
		func(req *http.Request) (*http.Response, error) {
			var err error
			mr, err = newMockedRequest(req.Clone(context.Background()))
			if err != nil {
				return nil, err
			}
			return httpmock.NewStringResponse(http.StatusOK, "Response"), nil
		},
	)

	resp, body := ts.sutRequest(http.MethodPost, _testAPIURL+"/main", map[string]interface{}{
		"text": "Test message",
	})
	defer resp.Body.Close()

	ts.Equal(http.StatusCreated, resp.StatusCode)
	ts.Equal(resp.Header.Get(_testContentTypeHeader), _testApplicationJSONCT)
	ts.JSONEq(
		`{
			"status": "success"
		}`,
		body,
	)

	time.Sleep(100 * time.Millisecond)

	ts.Equal(1, httpmock.GetTotalCallCount())

	ts.Require().NotNil(mr)
	ts.Equal(http.MethodPost, mr.Method)
	ts.Equal("https://api.telegram.org/botbot:token/sendMessage", mr.URL)
	ts.Equal("application/json", mr.Header.Get(_testContentTypeHeader))
	ts.JSONEq(
		`{
			"chat_id": "chat_1",
			"disable_notifications": 0,
			"disable_web_page_preview": 0,
			"text": "Test message"
		}`,
		mr.Body,
	)
}

func (ts *httpapiTestSuite) TestChannelIsFull() {
	httpmock.RegisterResponder(http.MethodPost, _testTelegamAPISendMessageMethod,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(http.StatusBadRequest, "Bad request"), nil
		},
	)

	err := ts.sut.StopChannel("main")
	ts.Require().NoError(err)
	time.Sleep(100 * time.Millisecond)

	ch, err := ts.sut.GetChannel("main")
	ts.Require().NoError(err)
	for i := 0; i < channels.TelegramMessageQueueCap; i++ {
		m := &channels.TelegramMessage{Text: "Test message"}
		err := ch.Enqueue(m)
		if err != nil {
			ts.FailNow(err.Error())
		}
	}

	resp, body := ts.sutRequest(http.MethodPost, _testAPIURL+"/main", map[string]interface{}{
		"text": "Test message",
	})
	defer resp.Body.Close()

	ts.Equal(http.StatusServiceUnavailable, resp.StatusCode)
	ts.Equal(resp.Header.Get(_testContentTypeHeader), _testApplicationJSONCT)
	ts.JSONEq(
		`{
			"status": "error",
			"message": "The channel is full"
		}`,
		body,
	)
}
