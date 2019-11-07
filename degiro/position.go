package degiro

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

type PositionCache struct {
	cache []Position
	mu    sync.RWMutex
}

func newPositionCache() PositionCache {
	return PositionCache{cache: []Position{}}
}

func (c *PositionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = []Position{}
}

func (c *PositionCache) Add(positions []Position) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = append(c.cache, positions...)
}

func (c *PositionCache) Update(positions []Position) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, position := range c.cache {
		for _, p := range positions {
			if position.ProductId == p.ProductId {
				position.Size = p.Size
				c.cache[i] = position
			}
		}
	}
}

func (c *PositionCache) Remove(positionsIds []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
cacheloop:
	for i := len(c.cache) - 1; i >= 0; i-- {
		for _, id := range positionsIds {
			if c.cache[i].ProductId == id {
				c.cache[i] = c.cache[len(c.cache)-1]
				c.cache = c.cache[:len(c.cache)-1]
				continue cacheloop
			}
		}
	}
}

func (c *PositionCache) Get(productId string) []Position {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var res []Position
	for _, position := range c.cache {
		if position.ProductId == productId {
			res = append(res, position)
		}
	}
	return res
}

type Position struct {
	ProductId string
	Size      int
}

type updatePositionsResponse struct {
	LastUpdated int                      `json:"lastUpdated"`
	Positions   []updatePositionResponse `json:"value"`
}

func (r updatePositionsResponse) ConvertToPositions() (added []Position, updated []Position, removed []string) {
	for _, position := range r.Positions {
		if position.IsRemoved {
			removed = append(removed, position.Id)
			continue
		}
		newPosition, err := position.ConvertToPosition()
		if err != nil {
			log.Warnf("converting response to position list: %v", err)
			continue
		}
		if position.IsAdded {
			added = append(added, newPosition)
			continue
		}
		updated = append(updated, newPosition)
	}
	return added, updated, removed
}

type updatePositionResponse struct {
	Id        string `json:"id"`
	IsAdded   bool   `json:"isAdded"`
	IsRemoved bool   `json:"isRemoved"`
	Value     []struct {
		Name  string      `json:"name"`
		Value interface{} `json:"value"`
	} `json:"value"`
}

func (r updatePositionResponse) ConvertToPosition() (Position, error) {
	res := Position{}
	res.ProductId = r.Id
	for _, property := range r.Value {
		switch property.Name {
		case "size":
			res.Size = int(property.Value.(float64))
		}
	}
	return res, nil
}

func (c *Client) GetOpenedPositionForProduct(productId string) (Position, bool) {
	positions := c.positions.Get(productId)
	if len(positions) == 0 {
		return Position{}, false
	}
	if positions[0].Size == 0 {
		return Position{}, false
	}
	return positions[0], true
}
