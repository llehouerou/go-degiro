package degiro

import (
	"sort"
	"time"

	"github.com/llehouerou/go-degiro/degiro/streaming"

	log "github.com/sirupsen/logrus"

	"github.com/shopspring/decimal"
)

type HistoricalPosition struct {
	ProductId    int
	transactions []Transaction
}

func (p *HistoricalPosition) AddTransaction(transaction Transaction) {
	p.transactions = append(p.transactions, transaction)
	sortTransactionsByDateAscending(p.transactions)
}

func sortHistoricalPositionByFirstTransactionDateAscending(hpositions []HistoricalPosition) {
	sort.SliceStable(hpositions, func(i, j int) bool {
		return hpositions[i].GetFirstTransactionDate().Before(hpositions[j].GetFirstTransactionDate())
	})
}

func (p *HistoricalPosition) GetTransactionCount() int {
	return len(p.transactions)
}

func (p *HistoricalPosition) GetSize() int {
	res := 0
	for _, t := range p.transactions {
		res += t.Quantity
	}
	return res
}

func GetPru(transactions []Transaction) decimal.Decimal {

	var totalSize int64
	var totalPrice decimal.Decimal
	for _, t := range transactions {
		if t.Quantity <= 0 {
			continue
		}
		totalSize += int64(t.Quantity)
		totalPrice = totalPrice.Sub(t.TotalPlusFeeInBaseCurrency)
	}
	if totalSize == 0 {
		return decimal.NewFromFloat(0)
	}
	return totalPrice.Div(decimal.New(totalSize, 0))
}

func (p *HistoricalPosition) GetPru() decimal.Decimal {
	return GetPru(p.transactions)
}

func (p *HistoricalPosition) GetPastPerformance() decimal.Decimal {

	var res decimal.Decimal
	var tmpTrans []Transaction
	for _, t := range p.transactions {
		if t.Quantity < 0 {
			currentPru := GetPru(tmpTrans)
			res = res.Add(t.Price.Sub(currentPru).Mul(decimal.New(int64(t.Quantity), 0).Abs()).Add(t.FeeInBaseCurrency))
		}
		tmpTrans = append(tmpTrans, t)
	}
	return res
}

func (p *HistoricalPosition) GetPastPerformanceInPercent() decimal.Decimal {
	return p.GetPastPerformance().Mul(decimal.New(100, 0)).Div(p.GetTotalBuyAmount())
}

func (p *HistoricalPosition) GetTotalBuyAmount() decimal.Decimal {
	var res decimal.Decimal
	for _, transaction := range p.transactions {
		if transaction.Quantity > 0 {
			res = res.Add(decimal.New(int64(transaction.Quantity), 0).Mul(transaction.Price))
		}
	}
	return res
}

func (p *HistoricalPosition) GetPastPerformanceSince(since time.Time) decimal.Decimal {

	var res decimal.Decimal
	var tmpTrans []Transaction
	for _, t := range p.transactions {
		if t.Quantity < 0 && since.Before(t.Date) {
			currentPru := GetPru(tmpTrans)
			res = res.Add(t.Price.Sub(currentPru).Mul(decimal.New(int64(t.Quantity), 0).Abs()).Add(t.FeeInBaseCurrency))
		}
		tmpTrans = append(tmpTrans, t)
	}
	return res
}

func (p *HistoricalPosition) GetCurrentPerformance(quote streaming.ProductQuote) decimal.Decimal {
	if quote.BidPrice.Equal(decimal.NewFromFloat(0)) || quote.AskPrice.Equal(decimal.NewFromFloat(0)) {
		return decimal.Decimal{}
	}
	return quote.AskPrice.Add(quote.BidPrice).Div(decimal.New(2, 0)).Sub(p.GetPru()).Mul(decimal.New(int64(p.GetSize()), 0))
}

func (p *HistoricalPosition) GetCurrentPerformanceInPercent(quote streaming.ProductQuote) decimal.Decimal {
	if quote.BidPrice.Equal(decimal.NewFromFloat(0)) || quote.AskPrice.Equal(decimal.NewFromFloat(0)) {
		return decimal.Decimal{}
	}
	if p.GetPru().Equal(decimal.Decimal{}) {
		return decimal.Decimal{}
	}
	return quote.AskPrice.Add(quote.BidPrice).Div(decimal.New(2, 0)).Sub(p.GetPru()).Mul(decimal.New(100, 0)).Div(p.GetPru())
}

func (p *HistoricalPosition) GetFirstTransactionDate() time.Time {

	if len(p.transactions) == 0 {
		return time.Time{}
	}
	return p.transactions[0].Date
}

func (p *HistoricalPosition) GetLastTransactionDate() time.Time {

	if len(p.transactions) == 0 {
		return time.Time{}
	}
	return p.transactions[len(p.transactions)-1].Date
}

func getHistoricalPositionsFromTransactions(transactions []Transaction) []HistoricalPosition {
	tmap := make(map[int][]Transaction)
	sortTransactionsByDateAscending(transactions)
	for _, t := range transactions {
		tmap[t.ProductId] = append(tmap[t.ProductId], t)
	}
	var tmp []*HistoricalPosition
	for pid, translist := range tmap {
		p := &HistoricalPosition{
			ProductId: pid,
		}
		tmp = append(tmp, p)
		for _, t := range translist {
			p.AddTransaction(t)
			if p.GetSize() != 0 {
				continue
			}
			p = &HistoricalPosition{
				ProductId: pid,
			}
			tmp = append(tmp, p)
		}
	}

	var res []HistoricalPosition
	for _, p := range tmp {
		if p.GetTransactionCount() > 0 {
			res = append(res, *p)
		}
	}
	sortHistoricalPositionByFirstTransactionDateAscending(res)
	return res
}

func (c *Client) startHistoricalPositionUdpating() {
	go func() {
		transactions, err := c.GetTransactions(time.Time{}, time.Now())
		if err != nil {
			log.Warnf("error while getting initial transaction history: %v", err)
		}
		c.transactions.Merge(transactions)
		ticker := time.NewTicker(c.HistoricalPositionUpdatePeriod)
		for {
			select {
			case <-ticker.C:
				transactions, err := c.GetTransactions(time.Now().Add(-c.HistoricalPositionUpdatePeriod-(1*time.Minute)), time.Now())
				if err != nil {
					log.Warnf("error while getting transaction history update: %v", err)
				}
				c.transactions.Merge(transactions)
			}
		}
	}()
}

func (c *Client) GetOpenedHistoricalPositionForProduct(productid string) (HistoricalPosition, bool) {
	return c.transactions.GetOpenedHistoricalPositionForProduct(productid)
}

func (c *Client) GetAllHistoricalPositions() []HistoricalPosition {
	return c.transactions.GetAllHistoricalPositions()
}
