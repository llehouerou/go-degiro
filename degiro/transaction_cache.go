package degiro

import (
	"strconv"
	"sync"
)

type TransactionCache struct {
	sync.RWMutex
	transactions []Transaction
	positions    []HistoricalPosition
}

func newTransactionCache() *TransactionCache {
	return &TransactionCache{
		RWMutex:      sync.RWMutex{},
		transactions: []Transaction{},
		positions:    []HistoricalPosition{},
	}
}

func (c *TransactionCache) Merge(transactions []Transaction) {
	c.Lock()
	defer c.Unlock()
	for _, transaction := range transactions {
		found := false
		for _, t := range c.transactions {
			if t.Id == transaction.Id {
				found = true
				break
			}
		}
		if !found {
			c.transactions = append(c.transactions, transaction)
		}
	}
	c.positions = getHistoricalPositionsFromTransactions(c.transactions)
}

func (c *TransactionCache) GetOpenedHistoricalPositionForProduct(productid string) (HistoricalPosition, bool) {
	c.RLock()
	defer c.RUnlock()
	productidInt, err := strconv.Atoi(productid)
	if err != nil {
		return HistoricalPosition{}, false
	}

	for _, position := range c.positions {
		if position.ProductId == productidInt && position.GetSize() > 0 {
			return position, true
		}
	}
	return HistoricalPosition{}, false
}

func (c *TransactionCache) GetHistoricalPositionsForProduct(productid string) []HistoricalPosition {
	c.RLock()
	defer c.RUnlock()
	productidInt, err := strconv.Atoi(productid)
	if err != nil {
		return []HistoricalPosition{}
	}

	var res []HistoricalPosition

	for _, position := range c.positions {
		if position.ProductId == productidInt {
			res = append(res, position)
		}
	}
	return res
}

func (c *TransactionCache) GetAllHistoricalPositions() []HistoricalPosition {
	c.RLock()
	defer c.RUnlock()
	var res []HistoricalPosition
	return append(res, c.positions...)
}
