package degiro

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"

	"github.com/dghubble/sling"
	"github.com/llehouerou/go-degiro/degiro/streaming"
	log "github.com/sirupsen/logrus"
)

const baseUrl = "https://trader.degiro.nl"

type Client struct {
	UpdatePeriod                   time.Duration
	StreamingUpdatePeriod          time.Duration
	TryReloginOn401                bool
	HistoricalPositionUpdatePeriod time.Duration

	httpclient      *http.Client
	sling           *sling.Sling
	streamingClient *streaming.Client

	username string
	password string

	clientId          int
	accountId         int64
	sessionId         string
	configuration     *Configuration
	userConfiguration *UserConfiguration

	ordersLastUpdate         int
	orders                   OrderCache
	portfolioLastUpdate      int
	positions                PositionCache
	totalPortfolioLastUpdate int
	balance                  BalanceCache

	transactions *TransactionCache
	products     *ProductCache

	reloginMu     sync.Mutex
	lastLoginDate time.Time
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient.Jar == nil {
		cookieJar, _ := cookiejar.New(nil)
		httpClient.Jar = cookieJar
	}

	base := sling.New().Client(httpClient).Base(baseUrl).New().
		Set("Origin", baseUrl).
		Set("Accept", "application/json, text/plain, */*").
		Set("Accept-Language", "fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3").
		Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:69.0) Gecko/20100101 Firefox/69.0").
		Set("Referer", "https://trader.degiro.nl/trader/")

	client := &Client{
		sling:                          base,
		httpclient:                     httpClient,
		orders:                         newOrderCache(),
		positions:                      newPositionCache(),
		balance:                        newBalanceCache(),
		UpdatePeriod:                   2 * time.Second,
		StreamingUpdatePeriod:          1 * time.Second,
		HistoricalPositionUpdatePeriod: 1 * time.Minute,
		TryReloginOn401:                true,
		streamingClient:                nil,
		transactions:                   newTransactionCache(),
		reloginMu:                      sync.Mutex{},
	}
	client.products = newProductCache(client, 24*time.Hour)
	return client
}

type LoginParams struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	IsRedirectToMobile bool   `json:"isRedirectToMobile"`
}

type LoginResponse struct {
	IsPassCodeEnabled bool   `json:"isPassCodeEnabled"`
	Locale            string `json:"locale"`
	RedirectUrl       string `json:"redirectUrl"`
	SessionId         string `json:"sessionId"`
	Status            int    `json:"status"`
	StatusText        string `json:"statusText"`
}

func (c *Client) Login(username string, password string) error {
	c.username = username
	c.password = password
	LoginResponse, err := c.login(username, password)
	if err != nil {
		return fmt.Errorf("login: %v", err)
	}
	c.sessionId = LoginResponse.SessionId
	c.configuration, err = c.getConfiguration()
	if err != nil {
		return fmt.Errorf("getting configuration: %v", err)
	}
	c.userConfiguration, err = c.getUserConfiguration()
	if err != nil {
		return fmt.Errorf("getting user configuration: %v", err)
	}
	c.accountId = c.userConfiguration.AccountId
	c.clientId = c.userConfiguration.ClientId
	c.startUpdating()
	c.startHistoricalPositionUdpating()
	c.streamingClient = streaming.NewStreamingClient(c.httpclient, c.clientId, c.StreamingUpdatePeriod)
	err = c.streamingClient.Start()
	if err != nil {
		return fmt.Errorf("starting streaming client: %v", err)
	}
	return nil
}

func (c *Client) login(username string, password string) (*LoginResponse, error) {
	LoginResponse := &LoginResponse{}
	resp, err := c.sling.New().Post("login/secure/login").
		Set("Referer", "https://trader.degiro.nl/login/fr").
		BodyJSON(&LoginParams{
			Username:           username,
			Password:           password,
			IsRedirectToMobile: false,
		}).ReceiveSuccess(LoginResponse)
	if err != nil {
		return nil, fmt.Errorf("request: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("not 2xx status code: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	c.lastLoginDate = time.Now()
	log.Infof("login ok > sessionId : %s", LoginResponse.SessionId)
	return LoginResponse, err
}

type Configuration struct {
	ClientId               int    `json:"clientId"`
	SessionId              string `json:"sessionId"`
	TradingUrl             string `json:"tradingUrl"`
	I18NUrl                string `json:"i18nUrl"`
	PaymentServiceUrl      string `json:"paymentServiceUrl"`
	ReportingUrl           string `json:"reportingUrl"`
	PaUrl                  string `json:"paUrl"`
	VwdQuotecastServiceUrl string `json:"vwdQuotecastServiceUrl"`
	ProductSearchUrl       string `json:"productSearchUrl"`
	DictionaryUrl          string `json:"dictionaryUrl"`
	TaskManagerUrl         string `json:"taskManagerUrl"`
	FirstLoginWizardUrl    string `json:"firstLoginWizardUrl"`
	VwdGossipsUrl          string `json:"vwdGossipsUrl"`
	CompaniesServiceUrl    string `json:"companiesServiceUrl"`
	ProductTypesUrl        string `json:"productTypesUrl"`
	VwdNewsUrl             string `json:"vwdNewsUrl"`
	LoginUrl               string `json:"loginUrl"`
}

func (c *Client) ReceiveSuccessReloginOn401(s *sling.Sling, successV interface{}) (*http.Response, error) {
	c.reloginMu.Lock()
	defer c.reloginMu.Unlock()

	resp, err := s.ReceiveSuccess(successV)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 && c.TryReloginOn401 && (time.Now().Sub(c.lastLoginDate) >= 15*time.Second) {
		log.Info("Try relogin")
		LoginResponse, err := c.login(c.username, c.password)
		if err != nil {
			return nil, fmt.Errorf("relogin on 401: %v", err)
		}
		c.sessionId = LoginResponse.SessionId
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("not 2xx HTTP status code: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return resp, nil
}

func (c *Client) getConfiguration() (*Configuration, error) {
	configuration := &Configuration{}
	resp, err := c.sling.New().Get("login/secure/config").
		ReceiveSuccess(configuration)
	if err != nil {
		return nil, fmt.Errorf("requesting configuration: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("requesting configuration: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return configuration, nil
}

type UserConfiguration struct {
	ClientId              int    `json:"id"`
	AccountId             int64  `json:"intAccount"`
	ClientRole            string `json:"clientRole"`
	EffectiveClientRole   string `json:"effectiveClientRole"`
	ContractType          string `json:"contractType"`
	Username              string `json:"username"`
	DisplayName           string `json:"displayName"`
	Email                 string `json:"email"`
	CellphoneNumber       string `json:"cellphoneNumber"`
	Locale                string `json:"locale"`
	Language              string `json:"language"`
	Culture               string `json:"culture"`
	MemberCode            string `json:"memberCode"`
	CanUpgrade            bool   `json:"canUpgrade"`
	IsAllocationAvailable bool   `json:"isAllocationAvailable"`
	IsCollectivePortfolio bool   `json:"isCollectivePortfolio"`
	IsAmClientActive      bool   `json:"isAmClientActive"`
	IsIskClient           bool   `json:"isIskClient"`
	IsWithdrawalAvailable bool   `json:"isWithdrawalAvailable"`
	FirstContact          struct {
		FirstName      string `json:"firstName"`
		LastName       string `json:"lastName"`
		DisplayName    string `json:"displayName"`
		Nationality    string `json:"nationality"`
		Gender         string `json:"gender"`
		DateOfBirth    string `json:"dateOfBirth"`
		PlaceOfBirth   string `json:"placeOfBirth"`
		CountryOfBirth string `json:"countryOfBirth"`
		Birthday       string `json:"birthday"`
	} `json:"firstContact"`
	Address struct {
		StreetAddress       string `json:"streetAddress"`
		StreetAddressNumber string `json:"streetAddressNumber"`
		Zip                 string `json:"zip"`
		City                string `json:"city"`
		Country             string `json:"country"`
	} `json:"address"`
	BankAccount struct {
		Iban          string `json:"iban"`
		Bic           string `json:"bic"`
		Name          string `json:"name"`
		BankAccountId int    `json:"bankAccountId"`
	} `json:"bankAccount"`
}

func (c *Client) getUserConfiguration() (*UserConfiguration, error) {
	type UserConfigurationQueryParams struct {
		SessionId string `url:"sessionId"`
	}
	type UserConfigurationResponse struct {
		UserConfiguration UserConfiguration `json:"data"`
	}
	userConfigurationResponse := &UserConfigurationResponse{}
	resp, err := c.sling.New().Get("pa/secure/client").
		QueryStruct(&UserConfigurationQueryParams{
			SessionId: c.sessionId,
		}).ReceiveSuccess(userConfigurationResponse)
	if err != nil {
		return nil, fmt.Errorf("requesting user configuration: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("requesting user configuration: %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return &userConfigurationResponse.UserConfiguration, nil
}

func (c *Client) NewStreamingClient(httpclient *http.Client, updatePeriod time.Duration) *streaming.Client {

	return streaming.NewStreamingClient(httpclient, c.clientId, updatePeriod)
}

func (c *Client) SubscribeQuotes(idlist []string) error {
	if c.streamingClient == nil {
		return fmt.Errorf("streaming client is not initialized")
	}
	err := c.streamingClient.SubscribeQuotes(idlist)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) UnSubscribeQuotes(idlist []string) error {
	if c.streamingClient == nil {
		return fmt.Errorf("streaming client is not initialized")
	}
	err := c.streamingClient.UnSubscribeQuotes(idlist)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetQuote(productvwid string) streaming.ProductQuote {
	if c.streamingClient == nil {
		return streaming.ProductQuote{}
	}
	return c.streamingClient.GetQuote(productvwid)
}

func (c *Client) GetBalance() Balance {
	return c.balance.Get()
}
