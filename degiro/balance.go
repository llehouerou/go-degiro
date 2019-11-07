package degiro

import (
	"sync"

	"github.com/shopspring/decimal"
)

type BalanceCache struct {
	cache Balance
	mu    sync.RWMutex
}

func newBalanceCache() BalanceCache {
	return BalanceCache{cache: Balance{}}
}

func (c *BalanceCache) Set(balance Balance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = balance
}

func (c *BalanceCache) Get() Balance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}

type Balance struct {
	Cash                decimal.Decimal
	FreeSpaceNewInEuros decimal.Decimal
	ReportPortfValue    decimal.Decimal
	ReportNetliq        decimal.Decimal
}

type updateBalanceResponse struct {
	LastUpdated int `json:"lastUpdated"`
	Value       []struct {
		Name  string      `json:"name"`
		Value interface{} `json:"value"`
	} `json:"value"`
}

func (r updateBalanceResponse) ConvertToBalance() (Balance, bool) {
	if len(r.Value) == 0 {
		return Balance{}, false
	}
	res := Balance{}
	for _, property := range r.Value {
		switch property.Name {
		case "cash":
			res.Cash = decimal.NewFromFloat(property.Value.(float64))
		case "freeSpaceNew":
			res.FreeSpaceNewInEuros = decimal.NewFromFloat(property.Value.(map[string]interface{})["EUR"].(float64))
		case "reportPortfValue":
			res.ReportPortfValue = decimal.NewFromFloat(property.Value.(float64))
		case "reportNetliq":
			res.ReportNetliq = decimal.NewFromFloat(property.Value.(float64))
		}
	}
	return res, true
}
