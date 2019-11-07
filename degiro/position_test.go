package degiro

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPositionCache_Add(t *testing.T) {
	cache := newPositionCache()
	cache.Add([]Position{
		{
			ProductId: "1",
			Size:      10,
		},
		{
			ProductId: "2",
			Size:      20,
		},
	})
	pos := cache.Get("1")
	assert.Equal(t, 1, len(pos))
	pos2 := cache.Get("2")
	assert.Equal(t, 1, len(pos2))
}

func TestPositionCache_Clear(t *testing.T) {
	cache := newPositionCache()
	cache.Add([]Position{
		{
			ProductId: "1",
			Size:      10,
		},
	})
	pos := cache.Get("1")
	assert.Equal(t, 1, len(pos))
	assert.Equal(t, "1", pos[0].ProductId)

	cache.Clear()
	pos = cache.Get("1")
	assert.Equal(t, 0, len(pos))
}

func TestPositionCache_Remove(t *testing.T) {
	cache := newPositionCache()
	cache.Add([]Position{
		{
			ProductId: "1",
			Size:      10,
		},
		{
			ProductId: "2",
			Size:      20,
		},
	})
	pos := cache.Get("1")
	assert.Equal(t, 1, len(pos))
	assert.Equal(t, "1", pos[0].ProductId)

	cache.Remove([]string{"1", "2"})
	pos = cache.Get("1")
	assert.Equal(t, 0, len(pos))
}
