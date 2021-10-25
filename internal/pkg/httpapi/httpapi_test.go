package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"github.com/unhandled-exception/tgproxy-go/internal/pkg/channels"
	"github.com/unhandled-exception/tgproxy-go/internal/pkg/httpapi"
)

var (
	testChannels = []string{
		"telegram://bot:token@chat_1/main/?timeout=100",
		"telegram://bot2:token2@chat_2/second",
	}
)

type httpapiTestSuite struct {
	suite.Suite

	sut *httpapi.HTTPAPI
}

func TestHttpAPI(t *testing.T) {
	suite.Run(t, new(httpapiTestSuite))
}

func (ts *httpapiTestSuite) SetupTest() {
	logger := zerolog.New(ioutil.Discard)
	chs, err := channels.BuildChannelsFromURLS(testChannels, &logger)
	if err != nil {
		ts.FailNow("Failed to init test app: %e", err)
	}
	ts.sut = httpapi.NewHTTPAPI(chs, &logger)
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
	resp, body := ts.sutRequest("GET", "http://api/ping", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)
	ts.Equal(resp.Header.Get("Content-Type"), "application/json; charset=utf-8")
	ts.JSONEq(
		`{"status": "success"}`,
		body,
	)
}

func (ts *httpapiTestSuite) TestGetIndex() {
	resp, body := ts.sutRequest("GET", "http://api/", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode)
	ts.Equal(resp.Header.Get("Content-Type"), "application/json; charset=utf-8")
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
