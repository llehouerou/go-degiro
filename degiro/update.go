package degiro

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type updateResponse struct {
	Orders    updateOrdersResponse    `json:"orders"`
	Portfolio updatePositionsResponse `json:"portfolio"`
	Balance   updateBalanceResponse   `json:"totalPortfolio"`
}

func (c *Client) update() error {
	type updateParams struct {
		Orders         int `url:"orders"`
		Portfolio      int `url:"portfolio"`
		TotalPortfolio int `url:"totalPortfolio"`
	}
	response := &updateResponse{}
	_, err := c.ReceiveSuccessReloginOn401(c.sling.New().
		Get(fmt.Sprintf("trading/secure/v5/update/%d;jsessionid=%s", c.accountId, c.sessionId)).
		QueryStruct(updateParams{
			Orders:         c.ordersLastUpdate,
			Portfolio:      c.portfolioLastUpdate,
			TotalPortfolio: c.totalPortfolioLastUpdate,
		}), response)
	if err != nil {
		return fmt.Errorf("executing request: %v", err)
	}

	c.ordersLastUpdate = response.Orders.LastUpdated
	c.updateOrderCacheFromResponse(response)
	c.portfolioLastUpdate = response.Portfolio.LastUpdated
	c.updatePositionCacheFromResponse(response)
	if c.totalPortfolioLastUpdate != response.Balance.LastUpdated {
		c.totalPortfolioLastUpdate = response.Balance.LastUpdated
		if b, found := response.Balance.ConvertToBalance(); found {
			c.balance.Set(b)
		}
	}
	return nil
}

func (c *Client) updateOrderCacheFromResponse(response *updateResponse) {
	added, updated, removed := response.Orders.ConvertToOrders()
	c.orders.Add(added)
	c.orders.Update(updated)
	c.orders.Remove(removed)
}

func (c *Client) updatePositionCacheFromResponse(response *updateResponse) {
	added, updated, removed := response.Portfolio.ConvertToPositions()
	c.positions.Add(added)
	c.positions.Update(updated)
	c.positions.Remove(removed)
}

func (c *Client) startUpdating() {
	go func() {
		ticker := time.NewTicker(c.UpdatePeriod)
		for {
			select {
			case <-ticker.C:
				err := c.update()
				if err != nil {
					logrus.Errorf("updating: %v", err)
				}
			}
		}
	}()
}
