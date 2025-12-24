package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Config holds the Valkey configuration
type Config struct {
	Host     string
	Port     int
	Password string
	Database int
}

// Client wraps the Valkey client
type Client struct {
	client valkey.Client
	config Config
}

// NewClient creates a new Valkey client
func NewClient(config Config) (*Client, error) {
	address := fmt.Sprintf("%s:%d", config.Host, config.Port)

	clientOpts := valkey.ClientOption{
		InitAddress: []string{address},
		SelectDB:    config.Database,
	}

	if config.Password != "" {
		clientOpts.Password = config.Password
	}

	client, err := valkey.NewClient(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

// Ping checks if the Valkey server is reachable
func (c *Client) Ping(ctx context.Context) error {
	cmd := c.client.B().Ping().Build()
	return c.client.Do(ctx, cmd).Error()
}

// Set stores a key-value pair with optional expiration
func (c *Client) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	var cmd valkey.Completed
	if expiration > 0 {
		cmd = c.client.B().Set().Key(key).Value(value).Ex(expiration).Build()
	} else {
		cmd = c.client.B().Set().Key(key).Value(value).Build()
	}

	return c.client.Do(ctx, cmd).Error()
}

// Get retrieves a value by key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	cmd := c.client.B().Get().Key(key).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return "", result.Error()
	}

	return result.ToString()
}

// Del deletes one or more keys
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	cmd := c.client.B().Del().Key(keys...).Build()
	return c.client.Do(ctx, cmd).Error()
}

// HSet stores a field-value pair in a hash
func (c *Client) HSet(ctx context.Context, key string, field string, value string) error {
	cmd := c.client.B().Hset().Key(key).FieldValue().FieldValue(field, value).Build()
	return c.client.Do(ctx, cmd).Error()
}

// HGet retrieves a field value from a hash
func (c *Client) HGet(ctx context.Context, key string, field string) (string, error) {
	cmd := c.client.B().Hget().Key(key).Field(field).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return "", result.Error()
	}

	return result.ToString()
}

// HDel deletes one or more fields from a hash
func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {

	if len(fields) == 0 {
		return nil
	}

	cmd := c.client.B().Hdel().Key(key).Field(fields...).Build()
	return c.client.Do(ctx, cmd).Error()
}

// HGetAll retrieves all field-value pairs from a hash
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	cmd := c.client.B().Hgetall().Key(key).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return nil, result.Error()
	}

	return result.AsStrMap()
}

// Exists checks if one or more keys exist
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	cmd := c.client.B().Exists().Key(keys...).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return 0, result.Error()
	}

	return result.AsInt64()
}

// Keys returns all keys matching a pattern
func (c *Client) Keys(ctx context.Context, pattern string) ([]string, error) {
	cmd := c.client.B().Keys().Pattern(pattern).Build()
	result := c.client.Do(ctx, cmd)

	if result.Error() != nil {
		return nil, result.Error()
	}

	return result.AsStrSlice()
}

// Expire sets an expiration on a key
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	cmd := c.client.B().Expire().Key(key).Seconds(int64(expiration.Seconds())).Build()
	return c.client.Do(ctx, cmd).Error()
}

// Close closes the Valkey client connection
func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
}
