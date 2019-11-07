package streaming

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dghubble/sling"
	"github.com/shopspring/decimal"
)

const (
	streamingBaseUrl    = "https://degiro.quotecast.vwdservices.com/CORS/"
	streamingApiVersion = "1.0.20180305"
)

var vwdDatas = []string{
	"BidPrice",
	"AskPrice",
	"LastPrice",
	//"LastTime",
	"BidVolume",
	"AskVolume",
	//"CumulativeVolume",
	"OpenPrice",
	"HighPrice",
	"LowPrice",
	"FullName",
}

type Client struct {
	httpclient *http.Client
	sling      *sling.Sling
	baseURL    *url.URL

	clientId          int
	quoteUpdatePeriod time.Duration
	sessionId         string

	indexes       *IndexMap
	stringValues  *StringValueMap
	decimalValues *DecimalValueMap
}

func NewStreamingClient(httpClient *http.Client, clientId int, updatePeriod time.Duration) *Client {
	baseURL, _ := url.Parse(streamingBaseUrl)

	base := sling.New().Client(httpClient).Base(streamingBaseUrl).
		Set("Origin", "https://trader.degiro.nl").
		Set("Accept", "*/*").
		Set("Accept-Language", "fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3").
		Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:69.0) Gecko/20100101 Firefox/69.0")

	client := &Client{
		sling:             base,
		httpclient:        httpClient,
		baseURL:           baseURL,
		clientId:          clientId,
		quoteUpdatePeriod: updatePeriod,
		indexes:           NewIndexMap(),
		stringValues:      NewStringValueMap(),
		decimalValues:     NewDecimalValueMap(),
	}
	return client
}

func (c *Client) Start() error {
	err := c.getNewSessionId()
	if err != nil {
		return fmt.Errorf("setting new session Id: %v", err)
	}
	go c.loopUpdateQuotes()
	return nil
}

func (c *Client) getNewSessionId() error {
	newsessionId, err := c.requestSession()
	if err != nil {
		return fmt.Errorf("requesting session id: %v", err)
	}
	c.sessionId = newsessionId
	return nil
}

func (c *Client) SubscribeQuotes(idlist []string) error {
	err := c.subscribeProductQuotes(idlist)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) UnSubscribeQuotes(idlist []string) error {
	err := c.unSubscribeProductQuotes(idlist)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) loopUpdateQuotes() {
	ticker := time.NewTicker(c.quoteUpdatePeriod)
	for {
		select {
		case <-ticker.C:
			err := c.getQuoteUpdates()
			if err != nil {
				log.Errorf("retrieving quote updates: %v", err)
			}
		}
	}
}

func (c *Client) getQuoteStringValue(name string) string {
	id, exist := c.indexes.Get(name)
	if !exist {
		return ""
	}
	value, exist := c.stringValues.Get(id)
	if !exist {
		return ""
	}
	return value
}
func (c *Client) getQuoteDecimalValue(name string) decimal.Decimal {
	id, exist := c.indexes.Get(name)
	if !exist {
		return decimal.Decimal{}
	}
	value, exist := c.decimalValues.Get(id)
	if !exist {
		return decimal.Decimal{}
	}
	return value
}

func (c *Client) GetQuote(issueid string) ProductQuote {
	quote := ProductQuote{
		IssueId: issueid,
	}
	quote.FullName = c.getQuoteStringValue(fmt.Sprintf("%s.FullName", issueid))
	quote.AskPrice = c.getQuoteDecimalValue(fmt.Sprintf("%s.AskPrice", issueid))
	quote.BidPrice = c.getQuoteDecimalValue(fmt.Sprintf("%s.BidPrice", issueid))
	quote.LastPrice = c.getQuoteDecimalValue(fmt.Sprintf("%s.LastPrice", issueid))
	quote.BidVolume = c.getQuoteDecimalValue(fmt.Sprintf("%s.BidVolume", issueid))
	quote.AskVolume = c.getQuoteDecimalValue(fmt.Sprintf("%s.AskVolume", issueid))
	quote.OpenPrice = c.getQuoteDecimalValue(fmt.Sprintf("%s.OpenPrice", issueid))
	quote.HighPrice = c.getQuoteDecimalValue(fmt.Sprintf("%s.HighPrice", issueid))
	quote.LowPrice = c.getQuoteDecimalValue(fmt.Sprintf("%s.LowPrice", issueid))
	return quote
}

func (c *Client) requestSession() (string, error) {
	response := &struct {
		SessionId string `json:"sessionId"`
	}{}
	resp, err := c.sling.New().Post(fmt.Sprintf("request_session?version=%s&userToken=%d", streamingApiVersion, c.clientId)).
		BodyJSON(&struct {
			Referrer string `json:"referrer"`
		}{
			Referrer: "https://internal.degiro.eu",
		}).ReceiveSuccess(response)
	if err != nil {
		return "", fmt.Errorf("requesting session: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("requesting session: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return response.SessionId, nil
}

func (c *Client) subscribeProductQuotes(issueIdList []string) error {
	return c.postProductQuotes(issueIdList, true)
}

func (c *Client) unSubscribeProductQuotes(issueIdList []string) error {
	return c.postProductQuotes(issueIdList, false)
}

func (c *Client) postProductQuotes(issueIdList []string, subscribe bool) error {
	resp, err := c.sling.New().Post(fmt.Sprintf("%s", c.sessionId)).
		BodyJSON(&struct {
			Data string `json:"controlData"`
		}{
			Data: GetControlDataFromIssueIdList(issueIdList, subscribe),
		}).ReceiveSuccess(nil)
	if err != nil {
		return fmt.Errorf("posting product quotes: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("posting product quotes: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return nil
}

func GetControlDataFromIssueIdList(issueIdList []string, subscribe bool) string {
	res := ""
	for _, issueId := range issueIdList {
		for _, data := range vwdDatas {
			if subscribe {
				res += fmt.Sprintf("req(%s.%s);", issueId, data)
			} else {
				res += fmt.Sprintf("rel(%s.%s);", issueId, data)
			}
		}
	}
	return res
}

func (c *Client) getQuoteUpdates() error {
	response := &[]struct {
		Name  string        `json:"m"`
		Value []interface{} `json:"v"`
	}{}
	resp, err := c.sling.New().Get(fmt.Sprintf("%s", c.sessionId)).ReceiveSuccess(response)
	if err != nil {
		return fmt.Errorf("requesting quote updates: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("not 2xx status: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	for _, entry := range *response {
		switch entry.Name {
		case "a_req":
			c.indexes.Set(entry.Value[0].(string), int64(entry.Value[1].(float64)))
		case "un":
			c.decimalValues.Set(int64(entry.Value[0].(float64)), decimal.NewFromFloat(entry.Value[1].(float64)))
		case "us":
			c.stringValues.Set(int64(entry.Value[0].(float64)), entry.Value[1].(string))
		case "sr":
			err := c.getNewSessionId()
			if err != nil {
				return fmt.Errorf("getting new sessionid: %v", err)
			}
		}
	}
	return nil
}
