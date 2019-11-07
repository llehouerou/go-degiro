package streaming

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func getCommonStreamingHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("Server", "vwd QuoteCast 6.8.12(02-8084)")
	headers.Set("Content-Type", "application/json; charset=UTF-8")
	headers.Set("Cache-Control", "no-transform, no-store, no-cache")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "Thu, 1 Jan 1970 00:00:00 GMT")
	headers.Set("Connection", "keep-alive")
	headers.Set("Access-Control-Allow-Origin", "https://trader.degiro.nl")
	return headers
}

func TestRequestSession(t *testing.T) {
	assert := assert.New(t)

	clientId := 123456

	client := NewTestClient(func(req *http.Request) *http.Response {
		assert.Equal(req.URL.String(), fmt.Sprintf("https://degiro.quotecast.vwdservices.com/CORS/request_session?userToken=%d&version=%s", clientId, streamingApiVersion))

		headers := getCommonStreamingHeaders()
		headers.Set("Content-Length", "52")
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(`{"sessionId":"fdba16eb-d421-46a0-af14-1667394629e9"}`)),
			Header:     headers,
		}
	})

	streaming := NewStreamingClient(client, clientId, 10*time.Second)
	sessionId, err := streaming.requestSession()

	assert.Nil(err)
	assert.Equal(sessionId, "fdba16eb-d421-46a0-af14-1667394629e9")
}

func TestSubscribeProductQuotes(t *testing.T) {
	assert := assert.New(t)
	sessionId := "fdba16eb-d421-46a0-af14-1667394629e9"
	issueList := []string{
		"123456",
		"789456",
	}
	client := NewTestClient(func(req *http.Request) *http.Response {
		assert.Equal(req.URL.String(), fmt.Sprintf("https://degiro.quotecast.vwdservices.com/CORS/%s", sessionId))
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(req.Body)
		assert.Nil(err)
		body := buf.String()
		assert.Equal(fmt.Sprintf("{\"controlData\":\"%s\"}\n", GetControlDataFromIssueIdList(issueList, true)), body)

		headers := getCommonStreamingHeaders()
		headers.Set("Content-Length", "0")
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(``)),
			Header:     headers,
		}
	})

	streaming := NewStreamingClient(client, 0, 10*time.Second)
	streaming.sessionId = sessionId
	err := streaming.subscribeProductQuotes(issueList)

	assert.Nil(err)

}
