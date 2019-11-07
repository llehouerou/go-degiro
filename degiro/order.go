package degiro

import (
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type OrderCache struct {
	cache []Order
	mu    sync.RWMutex
}

func newOrderCache() OrderCache {
	return OrderCache{cache: []Order{}}
}

func (c *OrderCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = []Order{}
}

func (c *OrderCache) Add(orders []Order) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = append(c.cache, orders...)
}

func (c *OrderCache) Update(orders []Order) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, order := range c.cache {
		for _, o := range orders {
			if order.Id == o.Id {
				order.Quantity = o.Quantity
				order.Price = o.Price
				order.StopPrice = o.StopPrice
				order.TotalOrderValue = o.TotalOrderValue
				order.OrderType = o.OrderType
				order.TimeType = o.TimeType
				order.IsDeletable = o.IsDeletable
				order.IsModifiable = o.IsModifiable
				c.cache[i] = order
			}
		}
	}
}

func (c *OrderCache) Remove(orderIds []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
cacheloop:
	for i := len(c.cache) - 1; i >= 0; i-- {
		for _, id := range orderIds {
			if c.cache[i].Id == id {
				c.cache[i] = c.cache[len(c.cache)-1]
				c.cache = c.cache[:len(c.cache)-1]
				continue cacheloop
			}
		}
	}
}

func (c *OrderCache) Get(productId int) []Order {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var res []Order
	for _, order := range c.cache {
		if order.ProductId == productId {
			res = append(res, order)
		}
	}
	return res
}

type ActionType string

const (
	Buy  ActionType = "BUY"
	Sell ActionType = "SELL"
)

type OrderType int

const (
	Limited     OrderType = 0
	MarketOrder OrderType = 2
	StopLoss    OrderType = 3
	StopLimited OrderType = 1
)

type TimeType int

const (
	Day       TimeType = 1
	Permanent TimeType = 3
)

type Fee struct {
	Id       int             `json:"id"`
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"`
}

type checkOrderResponse struct {
	Data struct {
		ConfirmationId   string          `json:"confirmationId"`
		FreeSpaceNew     decimal.Decimal `json:"freeSpaceNew"`
		TransactionFees  []Fee           `json:"transactionFees"`
		TransactionTaxes []Fee           `json:"transactionTaxes"`
	} `json:"data"`
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
}

type PlaceOrderInput struct {
	BuySell   ActionType      `json:"buySell"`
	OrderType OrderType       `json:"orderType"`
	ProductId string          `json:"productId"`
	Quantity  int             `json:"size"`
	TimeType  TimeType        `json:"timeType"`
	Price     decimal.Decimal `json:"price"`
	StopPrice decimal.Decimal `json:"stopPrice"`
}

type placeOrderQueryParams struct {
	AccountId int64  `url:"intAccount"`
	SessionId string `url:"sessionId"`
}

func (c *Client) checkOrder(input PlaceOrderInput) (string, error) {
	checkOrderResponse := &checkOrderResponse{}
	_, err := c.ReceiveSuccessReloginOn401(c.sling.New().
		Post(fmt.Sprintf("trading/secure/v5/checkOrder;jsessionid=%s", c.sessionId)).
		QueryStruct(&placeOrderQueryParams{
			AccountId: c.accountId,
			SessionId: c.sessionId,
		}).
		BodyJSON(input), checkOrderResponse)
	if err != nil {
		return "", err
	}
	return checkOrderResponse.Data.ConfirmationId, nil
}

func (c *Client) confirmOrder(confirmationId string, input PlaceOrderInput) (string, error) {
	type ConfirmOrderResponse struct {
		Data struct {
			OrderId    string `json:"orderId"`
			Status     int    `json:"status"`
			StatusText string `json:"statusText"`
		} `json:"data"`
	}
	confirmOrderResponse := &ConfirmOrderResponse{}
	_, err := c.ReceiveSuccessReloginOn401(c.sling.New().
		Post(fmt.Sprintf("trading/secure/v5/order/%s;jsessionid=%s", confirmationId, c.sessionId)).
		QueryStruct(&placeOrderQueryParams{
			AccountId: c.accountId,
			SessionId: c.sessionId,
		}).
		BodyJSON(input), confirmOrderResponse)
	if err != nil {
		return "", err
	}
	return confirmOrderResponse.Data.OrderId, nil
}

func (c *Client) PlaceOrder(input PlaceOrderInput) (string, error) {
	confirmationId, err := c.checkOrder(input)
	if err != nil {
		return "", fmt.Errorf("checking order: %v", err)
	}
	orderId, err := c.confirmOrder(confirmationId, input)
	if err != nil {
		return "", fmt.Errorf("confirming order: %v", err)
	}
	return orderId, nil
}

func (c *Client) DeleteOrder(orderid string) error {
	_, err := c.ReceiveSuccessReloginOn401(c.sling.New().
		Delete(fmt.Sprintf("trading/secure/v5/order/%s;jsessionid=%s", orderid, c.sessionId)).
		QueryStruct(&placeOrderQueryParams{
			AccountId: c.accountId,
			SessionId: c.sessionId,
		}), nil)
	if err != nil {
		return err
	}
	return nil
}

type Order struct {
	Id              string
	Date            time.Time
	ProductId       int
	ProductName     string
	ContractType    int
	ContractSize    decimal.Decimal
	Currency        string
	BuySell         ActionType
	Size            int
	Quantity        int
	Price           decimal.Decimal
	StopPrice       decimal.Decimal
	TotalOrderValue decimal.Decimal
	OrderType       OrderType
	TimeType        TimeType
	IsModifiable    bool
	IsDeletable     bool
}

type updateOrdersResponse struct {
	LastUpdated int                   `json:"lastUpdated"`
	Value       []updateOrderResponse `json:"value"`
}

func (r updateOrdersResponse) ConvertToOrders() (added []Order, updated []Order, removed []string) {
	for _, order := range r.Value {
		if order.IsRemoved {
			removed = append(removed, order.OrderId)
			continue
		}
		newOrder, err := order.ConvertToOrder()
		if err != nil || newOrder == nil {
			continue
		}
		if order.IsAdded {
			added = append(added, *newOrder)
			continue
		}
		updated = append(updated, *newOrder)
	}
	return added, updated, removed
}

type updateOrderResponse struct {
	OrderId   string `json:"id"`
	IsAdded   bool   `json:"isAdded"`
	IsRemoved bool   `json:"isRemoved"`
	Value     []struct {
		Name  string      `json:"name"`
		Value interface{} `json:"value"`
	} `json:"value"`
}

func (r updateOrderResponse) ConvertToOrder() (*Order, error) {
	order := Order{}
	for _, property := range r.Value {
		order.Id = r.OrderId
		switch property.Name {
		case "productId":
			order.ProductId = int(property.Value.(float64))
		case "product":
			order.ProductName = property.Value.(string)
		case "buysell":
			var buysell ActionType
			buysell, err := convertShortActionType(property.Value.(string))
			if err != nil {
				return nil, fmt.Errorf("parsing buysell %s: %v", property.Value.(string), err)
			}
			order.BuySell = buysell
		case "size":
			order.Size = int(property.Value.(float64))
		case "quantity":
			order.Quantity = int(property.Value.(float64))
		case "price":
			order.Price = decimal.NewFromFloat(property.Value.(float64))
		case "stopPrice":
			order.StopPrice = decimal.NewFromFloat(property.Value.(float64))
		case "date":
			var date time.Time
			date, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", time.Now().Format("2006-01-02"), property.Value.(string)))
			if err != nil {
				date, err = time.Parse("02/01/2006", fmt.Sprintf("%s/%s", property.Value.(string), time.Now().Format("2006")))
				if err != nil {
					return nil, fmt.Errorf("parsing date %s: %v", property.Value.(string), err)
				}
			}
			order.Date = date
		case "contractType":
			order.ContractType = int(property.Value.(float64))
		case "contractSize":
			order.ContractSize = decimal.NewFromFloat(property.Value.(float64))
		case "currency":
			order.Currency = property.Value.(string)
		case "totalOrderValue":
			order.TotalOrderValue = decimal.NewFromFloat(property.Value.(float64))
		case "orderTypeId":
			order.OrderType = OrderType(property.Value.(float64))
		case "orderTimeTypeId":
			order.TimeType = TimeType(property.Value.(float64))
		case "isModifiable":
			order.IsModifiable = property.Value.(bool)
		case "isDeletable":
			order.IsDeletable = property.Value.(bool)
		}
	}
	return &order, nil
}

func (c *Client) GetPendingOrders(productId int) []Order {
	return c.orders.Get(productId)
}

func convertShortActionType(s string) (ActionType, error) {
	switch s {
	case "B":
		return Buy, nil
	case "S":
		return Sell, nil
	default:
		return "", fmt.Errorf("unknown action type: %s", s)

	}
}
