package degiro

import (
	"net/url"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	BuySell                    string          `json:"buysell"`
	Quantity                   int             `json:"quantity"`
	OrderType                  OrderType       `json:"orderTypeId"`
	CounterParty               string          `json:"counterParty"`
	TotalInBaseCurrency        decimal.Decimal `json:"totalInBaseCurrency"`
	FeeInBaseCurrency          decimal.Decimal `json:"feeInBaseCurrency"`
	TotalPlusFeeInBaseCurrency decimal.Decimal `json:"totalPlusFeeInBaseCurrency"`
	Transfered                 bool            `json:"transfered"`
	ProductId                  int             `json:"productId"`
	Price                      decimal.Decimal `json:"price"`
	Date                       time.Time       `json:"date"`
	Total                      decimal.Decimal `json:"total"`
	Id                         int             `json:"id"`
}

type shortDateTime time.Time

func (t shortDateTime) EncodeValues(key string, v *url.Values) error {
	v.Add(key, time.Time(t).Format("02/01/2006"))
	return nil
}

func (c *Client) GetTransactions(fromDate time.Time, toDate time.Time) ([]Transaction, error) {
	type getTransactionsResponse struct {
		Transactions []Transaction `json:"data"`
	}
	response := &getTransactionsResponse{}
	_, err := c.ReceiveSuccessReloginOn401(c.sling.New().
		Get("reporting/secure/v4/transactions").
		QueryStruct(&struct {
			FromDate  shortDateTime `url:"fromDate"`
			ToDate    shortDateTime `url:"toDate"`
			AccountId int64         `url:"intAccount"`
			SessionId string        `url:"sessionId"`
		}{
			FromDate:  shortDateTime(fromDate),
			ToDate:    shortDateTime(toDate),
			AccountId: c.accountId,
			SessionId: c.sessionId,
		}), response)
	if err != nil {
		return nil, err
	}
	return response.Transactions, nil
}

func sortTransactionsByDateAscending(transactions []Transaction) {
	sort.SliceStable(transactions, func(i, j int) bool {
		return transactions[i].Date.Before(transactions[j].Date)
	})
}
