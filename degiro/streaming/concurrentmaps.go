package streaming

import (
	"sync"

	"github.com/shopspring/decimal"
)

type IndexMap struct {
	sync.RWMutex
	items map[string]int64
}

func NewIndexMap() *IndexMap {
	return &IndexMap{
		items: make(map[string]int64),
	}
}

// Set adds an item to a concurrent map
func (m *IndexMap) Set(key string, value int64) {
	m.Lock()
	defer m.Unlock()
	m.items[key] = value
}

// Get retrieves the value for a concurrent map item
func (m *IndexMap) Get(key string) (int64, bool) {
	m.RLock()
	defer m.RUnlock()
	value, ok := m.items[key]
	return value, ok
}

type StringValueMap struct {
	sync.RWMutex
	items map[int64]string
}

func NewStringValueMap() *StringValueMap {
	return &StringValueMap{
		items: make(map[int64]string),
	}
}

// Set adds an item to a concurrent map
func (m *StringValueMap) Set(key int64, value string) {
	m.Lock()
	defer m.Unlock()
	m.items[key] = value
}

// Get retrieves the value for a concurrent map item
func (m *StringValueMap) Get(key int64) (string, bool) {
	m.RLock()
	defer m.RUnlock()
	value, ok := m.items[key]
	return value, ok
}

type DecimalValueMap struct {
	sync.RWMutex
	items map[int64]decimal.Decimal
}

func NewDecimalValueMap() *DecimalValueMap {
	return &DecimalValueMap{
		items: make(map[int64]decimal.Decimal),
	}
}

// Set adds an item to a concurrent map
func (m *DecimalValueMap) Set(key int64, value decimal.Decimal) {
	m.Lock()
	defer m.Unlock()
	m.items[key] = value
}

// Get retrieves the value for a concurrent map item
func (m *DecimalValueMap) Get(key int64) (decimal.Decimal, bool) {
	m.RLock()
	defer m.RUnlock()
	value, ok := m.items[key]
	return value, ok
}
