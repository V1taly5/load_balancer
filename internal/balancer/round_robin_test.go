package balancer

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundRobinBalancer(t *testing.T) {
	createBackend := func(u string, down bool) *Backend {
		parsedURL, _ := url.Parse(u)
		return &Backend{
			URL:    parsedURL,
			isDown: down,
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))

	t.Run("No backends available", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		_, err := rr.Next()
		assert.ErrorIs(t, err, ErrNoAvailableBackends)
	})

	t.Run("All backends down", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		rr.AddBackend(*createBackend("http://server1.com", true))
		rr.AddBackend(*createBackend("http://server2.com", true))

		_, err := rr.Next()
		assert.ErrorIs(t, err, ErrNoAvailableBackends)
	})

	t.Run("Round Robin order", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		server1 := createBackend("http://server1.com", false)
		server2 := createBackend("http://server2.com", false)
		server3 := createBackend("http://server3.com", false)

		rr.AddBackend(*server1)
		rr.AddBackend(*server2)
		rr.AddBackend(*server3)

		b1, _ := rr.Next()
		assert.Equal(t, server1.URL, b1.URL)

		b2, _ := rr.Next()
		assert.Equal(t, server2.URL, b2.URL)

		b3, _ := rr.Next()
		assert.Equal(t, server3.URL, b3.URL)

		b4, _ := rr.Next()
		assert.Equal(t, server1.URL, b4.URL)
	})

	t.Run("Skip down backends", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		server1 := createBackend("http://server1.com", false)
		server2 := createBackend("http://server2.com", true) // Down
		server3 := createBackend("http://server3.com", false)

		rr.AddBackend(*server1)
		rr.AddBackend(*server2)
		rr.AddBackend(*server3)

		b1, _ := rr.Next()
		assert.Equal(t, server1.URL, b1.URL)

		b2, _ := rr.Next()
		assert.Equal(t, server3.URL, b2.URL)

		b3, _ := rr.Next()
		assert.Equal(t, server1.URL, b3.URL)
	})

	t.Run("Add and Remove backends", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		server := createBackend("http://server1.com", false)

		rr.AddBackend(*server)
		assert.Len(t, rr.GetAllBackends(), 1)

		removed := rr.RemoveBackend(server.URL.String())
		assert.True(t, removed)
		assert.Len(t, rr.GetAllBackends(), 0)

		removed = rr.RemoveBackend("http://invalid.com")
		assert.False(t, removed)
	})

	t.Run("MarkAsDown", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		server := createBackend("http://server1.com", false)

		rr.MarkAsDown(server)
		assert.True(t, server.IsDown())

		rr.MarkAsDown(nil)
	})

	t.Run("GetAllBackends copy", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		server := createBackend("http://server1.com", false)
		rr.AddBackend(*server)

		backends := rr.GetAllBackends()
		backends[0] = nil

		assert.NotNil(t, rr.GetAllBackends()[0])
	})

	t.Run("Concurrency", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		var wg sync.WaitGroup
		const numOps = 100

		createURL := func(i int) string {
			return fmt.Sprintf("http://server%d.test", i)
		}

		for i := 0; i < numOps; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				url := createURL(i)
				server := createBackend(url, false)
				rr.AddBackend(*server)
			}(i)
		}

		for i := 0; i < numOps; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = rr.Next()
			}()
		}

		for i := 0; i < numOps; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				url := createURL(i)
				rr.RemoveBackend(url)
			}(i)
		}

		wg.Wait()
	})

	t.Run("SetHealth check", func(t *testing.T) {
		rr := NewRoundRobinBalancer(logger)
		server := createBackend("http://server1.com", true)
		rr.AddBackend(*server)

		server.SetHealth(false)

		assert.False(t, server.IsDown())
	})
}
