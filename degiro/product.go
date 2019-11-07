package degiro

import (
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

type ProductType int

const (
	Fund      ProductType = 13
	Leveraged ProductType = 14
	Index     ProductType = 180
	Etf       ProductType = 131
	Stock     ProductType = 1
	Bond      ProductType = 2
	Cash      ProductType = 311
	Currency  ProductType = 3
	Future    ProductType = 7
	Cfd       ProductType = 535
	Warrant   ProductType = 536
	Option    ProductType = 8
)

type productTime struct {
	time.Time
}

func (t *productTime) UnmarshalJSON(buf []byte) error {
	tt, err := time.Parse("2006-01-02", strings.Trim(string(buf), `"`))
	if err != nil {
		tt, err = time.Parse("2-1-2006", strings.Trim(string(buf), `"`))
		if err != nil {
			return err
		}
	}
	t.Time = tt
	return nil
}

type ProductCacheItem struct {
	Product    Product
	LastUpdate time.Time
}

type ProductCache struct {
	sync.RWMutex
	cache                     []ProductCacheItem
	client                    *Client
	CacheInvalidationDuration time.Duration
	productsToUpdate          map[string]bool
	updateLock                sync.Mutex
}

func newProductCache(client *Client, invalidationDuration time.Duration) *ProductCache {
	cache := &ProductCache{
		RWMutex:                   sync.RWMutex{},
		cache:                     []ProductCacheItem{},
		client:                    client,
		CacheInvalidationDuration: invalidationDuration,
		updateLock:                sync.Mutex{},
		productsToUpdate:          make(map[string]bool),
	}
	go cache.updateCache()
	return cache

}

func (c *ProductCache) get(productid string) (ProductCacheItem, bool) {
	c.RLock()
	defer c.RUnlock()
	for _, item := range c.cache {
		if item.Product.Id == productid {
			return item, true
		}
	}
	return ProductCacheItem{}, false
}
func (c *ProductCache) remove(productid string) {
	c.Lock()
	defer c.Unlock()
	for i := len(c.cache) - 1; i >= 0; i-- {
		if c.cache[i].Product.Id == productid {
			c.cache[i] = c.cache[len(c.cache)-1]
			c.cache = c.cache[:len(c.cache)-1]
		}
	}
}

func (c *ProductCache) add(product Product) {
	c.Lock()
	defer c.Unlock()
	found := false
	for i, item := range c.cache {
		if item.Product.Id == product.Id {
			c.cache[i].Product = product
			c.cache[i].LastUpdate = time.Now()
			found = true
		}
	}
	if !found {
		c.cache = append(c.cache, ProductCacheItem{
			Product:    product,
			LastUpdate: time.Now(),
		})
	}
}

func (c *ProductCache) updateCache() {
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticker.C:
			func() {
				c.updateLock.Lock()
				defer c.updateLock.Unlock()
				if len(c.productsToUpdate) == 0 {
					return
				}
				var ids []string
				for s := range c.productsToUpdate {
					ids = append(ids, s)
				}
				newProducts, err := c.client.getProducts(ids)
				if err != nil {
					log.Warnf("error while updating product infos: %v", err)
				}
				for _, product := range newProducts {
					c.add(product)
				}
				c.productsToUpdate = make(map[string]bool)
			}()
		}

	}
}

func (c *ProductCache) GetProducts(productids []string) []Product {
	var productsInCache []Product
	for i := len(productids) - 1; i >= 0; i-- {
		item, ok := c.get(productids[i])
		if !ok {
			continue
		}
		if time.Now().Sub(item.LastUpdate) > c.CacheInvalidationDuration {
			func() {
				c.updateLock.Lock()
				defer c.updateLock.Unlock()
				c.productsToUpdate[productids[i]] = true
			}()
		}
		productsInCache = append(productsInCache, item.Product)
		productids[i] = productids[len(productids)-1]
		productids = productids[:len(productids)-1]
	}

	if len(productids) == 0 {
		return productsInCache
	}
	newProducts, err := c.client.getProducts(productids)
	if err != nil {
		log.Warnf("error while getting product infos: %v", err)
	}
	for _, product := range newProducts {
		c.add(product)
	}
	return append(productsInCache, newProducts...)
}

func (c *ProductCache) GetProduct(productId string) (Product, bool) {
	products := c.GetProducts([]string{productId})
	if len(products) == 0 {
		return Product{}, false
	}
	return products[0], true
}

type Product struct {
	Id                       string          `json:"id"`
	Name                     string          `json:"name"`
	Isin                     string          `json:"isin"`
	Symbol                   string          `json:"symbol"`
	ContractSize             decimal.Decimal `json:"contractSize"`
	ProductTypeName          string          `json:"productType"`
	ProductTypeId            int             `json:"productTypeId"`
	Tradable                 bool            `json:"tradable"`
	Category                 string          `json:"category"`
	Currency                 string          `json:"currency"`
	StrikePrice              decimal.Decimal `json:"strikePrice"`
	ExchangeId               string          `json:"exchangeId"`
	TimeTypes                []string        `json:"orderTimeTypes"`
	GtcAllowed               bool            `json:"gtcAllowed"`
	BuyOrderTypes            []string        `json:"buyOrderTypes"`
	SellOrderTypes           []string        `json:"sellOrderTypes"`
	MarketAllowed            bool            `json:"marketAllowed"`
	LimitHitOrderAllowed     bool            `json:"limitHitOrderAllowed"`
	StopLossAllowed          bool            `json:"stoplossAllowed"`
	StopLimitOrderAllowed    bool            `json:"stopLimitOrderAllowed"`
	JoinOrderAllowed         bool            `json:"joinOrderAllowed"`
	TrailingStopOrderAllowed bool            `json:"trailingStopOrderAllowed"`
	CombinedOrderAllowed     bool            `json:"combinedOrderAllowed"`
	SellAmountAllowed        bool            `json:"sellAmountAllowed"`
	IsFund                   bool            `json:"isFund"`
	ClosePrice               decimal.Decimal `json:"closePrice"`
	ClosePriceDate           productTime     `json:"closePriceDate"`
	FeedQuality              string          `json:"feedQuality"`
	OrderBookDepth           int             `json:"orderBookDepth"`
	VwdIdentifierType        string          `json:"vwdIdentifierType"`
	VwdId                    string          `json:"vwdId"`
	QualitySwitchable        bool            `json:"qualitySwitchable"`
	QualitySwitchFree        bool            `json:"qualitySwitchFree"`
	VwdModuleId              int             `json:"vwdModuleId"`
	ExpirationDate           productTime     `json:"expirationDate"`
	FinancingLevel           decimal.Decimal `json:"financingLevel"`
	Leverage                 decimal.Decimal `json:"leverage"`
	Ratio                    decimal.Decimal `json:"ratio"`
	ShortLong                string          `json:"shortlong"`
	StopLoss                 decimal.Decimal `json:"stoploss"`
}

type SearchProductsOptions struct {
	SearchText  string
	Limit       int
	ProductType ProductType
}

func (c *Client) SearchProducts(options SearchProductsOptions) ([]Product, error) {
	type searchProductResponse struct {
		Offset   int       `json:"offset"`
		Products []Product `json:"products"`
	}
	response := &searchProductResponse{}
	_, err := c.ReceiveSuccessReloginOn401(c.sling.New().
		Get("product_search/secure/v5/products/lookup").
		QueryStruct(&struct {
			AccountId   int64       `url:"intAccount"`
			SessionId   string      `url:"sessionId"`
			SearchText  string      `url:"searchText"`
			Limit       int         `url:"limit"`
			ProductType ProductType `url:"productTypeId,omitempty"`
		}{
			AccountId:   c.accountId,
			SessionId:   c.sessionId,
			SearchText:  options.SearchText,
			Limit:       options.Limit,
			ProductType: options.ProductType,
		}), response)
	if err != nil {
		return nil, err
	}
	return response.Products, nil
}

func (c *Client) SearchProduct(searchtext string) (*Product, bool, error) {
	products, err := c.SearchProducts(SearchProductsOptions{
		SearchText: searchtext,
		Limit:      1,
	})
	if err != nil {
		return nil, false, err
	}
	if len(products) == 0 {
		return nil, false, nil
	}
	return &products[0], true, nil
}

func (c *Client) getProducts(productIds []string) ([]Product, error) {
	type getProductResponse struct {
		Products map[int64]Product `json:"data"`
	}
	response := &getProductResponse{}
	_, err := c.ReceiveSuccessReloginOn401(c.sling.New().
		Post("product_search/secure/v5/products/info").
		QueryStruct(&struct {
			AccountId int64  `url:"intAccount"`
			SessionId string `url:"sessionId"`
		}{
			AccountId: c.accountId,
			SessionId: c.sessionId,
		}).BodyJSON(productIds), response)
	if err != nil {
		return nil, err
	}
	var res []Product
	for _, product := range response.Products {
		res = append(res, product)
	}
	return res, nil
}

func (c *Client) GetProducts(productIds []string) []Product {
	return c.products.GetProducts(productIds)
}

func (c *Client) GetProduct(productId string) (Product, bool) {
	return c.products.GetProduct(productId)
}
