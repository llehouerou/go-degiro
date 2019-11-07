package degiro

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

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

func TestLogin(t *testing.T) {

	assert := assert.New(t)

	client := NewTestClient(func(req *http.Request) *http.Response {
		assert.Equal(req.URL.String(), "https://trader.degiro.nl/login/secure/login")

		headers := getCommonHeaders()
		headers.Set("Set-Cookie", "JSESSIONID=FE1544EE1A2905C0954F71F863DA7EC2.prod11; Path=/; HttpOnly")
		headers.Set("Access-Control-Allow-Origin", "https://trader.degiro.nl")
		headers.Set("Content-Length", "180")
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(`{"isPassCodeEnabled":true,"locale":"fr_FR","redirectUrl":"https://trader.degiro.nl/trader/","sessionId":"FE1544EE1A2905C0954F71F863DA7EC2.prod11","status":0,"statusText":"success"}`)),
			Header:     getCommonHeaders(),
		}
	})

	degiro := NewClient(client)
	resp, err := degiro.login("login", "password")

	assert.Nil(err)
	if assert.NotNil(resp) {
		assert.Equal("FE1544EE1A2905C0954F71F863DA7EC2.prod11", resp.SessionId)
		assert.Equal(0, resp.Status)
		assert.Equal("success", resp.StatusText)
		assert.Equal("https://trader.degiro.nl/trader/", resp.RedirectUrl)
		assert.Equal(true, resp.IsPassCodeEnabled)
		assert.Equal("fr_FR", resp.Locale)
	}

}

func getCommonHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("Server", "openresty")
	headers.Set("Date", "Mon, 30 Sep 2019 10:19:51 GMT")
	headers.Set("Content-Type", "application/json;charset=UTF-8")
	headers.Set("Connection", "keep-alive")
	headers.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")
	headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	headers.Set("Access-Control-Allow-Credentials", "true")
	return headers
}

func TestGetUserConfiguration(t *testing.T) {
	assert := assert.New(t)
	sessionId := "FE1544EE1A2905C0954F71F863DA7EC2.prod11"
	client := NewTestClient(func(req *http.Request) *http.Response {
		assert.Equal(req.URL.String(), fmt.Sprintf("https://trader.degiro.nl/pa/secure/client?sessionId=%s", sessionId))

		headers := getCommonHeaders()
		headers.Set("Content-Length", "979")
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString(`{"data":{"id":123456,"intAccount":12345678,"clientRole":"active","effectiveClientRole":"active","contractType":"PRIVATE","username":"username","displayName":"displayName","email":"foo@bar.com","firstContact":{"firstName":"Foo","lastName":"Bar","displayName":"Foo Bar","nationality":"US","gender":"MALE","dateOfBirth":"1900-01-01","placeOfBirth":"","countryOfBirth":"US"},"address":{"streetAddress":"","streetAddressNumber":"","zip":"","city":"","country":""},"cellphoneNumber":"","locale":"","language":"","culture":"","bankAccount":{"bankAccountId":0,"bic":"","name":"","iban":"","status":"VERIFIED"},"memberCode":"","isWithdrawalAvailable":true,"isAllocationAvailable":true,"isIskClient":false,"isCollectivePortfolio":false,"isAmClientActive":false,"canUpgrade":true}}`)),
			Header:     headers,
		}
	})
	degiro := NewClient(client)
	degiro.sessionId = sessionId
	config, err := degiro.getUserConfiguration()

	assert.Nil(err)
	if assert.NotNil(config) {
		assert.Equal(int64(12345678), config.AccountId)
		assert.Equal(123456, config.ClientId)
		assert.Equal("username", config.Username)
	}
}
