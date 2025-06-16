package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, err error, args ...any)
}

type Cache struct {
	log    Logger
	client *redis.Client
}

func New(log Logger, client *redis.Client) *Cache {
	return &Cache{log: log, client: client}
}

func (c *Cache) GetAll(pattern string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const (
		scanCount = 100
		workers   = 10
	)

	keyChan := make(chan string, 1000)
	result := make(map[string]interface{})
	var resultMu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range keyChan {
				var value interface{}
				if err := c.Get(key, &value); err == nil {
					resultMu.Lock()
					result[key] = value
					resultMu.Unlock()
				} else if !errors.Is(err, redis.Nil) {
					c.log.Error("Failed to get key", err, slog.String("key", key))
				}
			}
		}()
	}

	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			c.log.Error("Redis SCAN failed", err, slog.String("pattern", pattern))
			break
		}

		for _, key := range keys {
			select {
			case keyChan <- key:
			case <-ctx.Done():
				break
			}
		}

		cursor = nextCursor
		if cursor == 0 || ctx.Err() != nil {
			break
		}
	}

	close(keyChan)
	wg.Wait()

	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	return result, nil
}

func (c *Cache) Get(key string, dest any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if data != "" {
			c.log.Error("Failed to get redis data", err, slog.String("key", key), slog.Any("dest", dest))
		}
		return err
	}

	return json.Unmarshal([]byte(data), dest)
}

func (c *Cache) Set(key string, value any, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := json.Marshal(value)
	if err != nil {
		c.log.Error("Failed to marshal data to json", err, slog.String("key", key), slog.Any("value", value), slog.Duration("ttl", ttl))
		return
	}

	err = c.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		c.log.Error("Failed to set key", err, slog.String("key", key), slog.Duration("ttl", ttl))
	}
}

func (c *Cache) Delete(keys ...string) {
	if len(keys) == 0 {
		return
	}

	var keysToDelete []string
	for _, pattern := range keys {
		if strings.ContainsAny(pattern, "*?[\\") {
			iter := c.client.Scan(context.Background(), 0, pattern, 0).Iterator()
			for iter.Next(context.Background()) {
				keysToDelete = append(keysToDelete, iter.Val())
			}
			if err := iter.Err(); err != nil {
				c.log.Error("SCAN error", err, slog.String("pattern", pattern))
			}
		} else {
			keysToDelete = append(keysToDelete, pattern)
		}
	}

	if len(keysToDelete) > 0 {
		err := c.client.Del(context.Background(), keysToDelete...).Err()
		if err != nil {
			c.log.Error("Failed to delete keys", err, slog.Any("keys", keysToDelete))
		}
	}
}
