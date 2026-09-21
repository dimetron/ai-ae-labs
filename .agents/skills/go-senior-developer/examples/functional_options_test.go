package examples_test

import (
	"errors"
	"testing"
	"time"
)

// Configurable HTTP Client demonstrating the Functional Options pattern.
type Client struct {
	baseURL string
	timeout time.Duration
	retries int
	headers map[string]string
}

// Option modifies client configuration.
type Option func(*Client) error

// WithTimeout configures a custom request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if d <= 0 {
			return errors.New("timeout must be greater than zero")
		}
		c.timeout = d
		return nil
	}
}

// WithRetries configures retry count with validation.
func WithRetries(n int) Option {
	return func(c *Client) error {
		if n < 0 {
			return errors.New("retries cannot be negative")
		}
		c.retries = n
		return nil
	}
}

// WithHeader adds a default header.
func WithHeader(key, value string) Option {
	return func(c *Client) error {
		if key == "" {
			return errors.New("header key cannot be empty")
		}
		c.headers[key] = value
		return nil
	}
}

// NewClient initializes a Client with sensible defaults and optional functional overrides.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("baseURL is required")
	}

	c := &Client{
		baseURL: baseURL,
		timeout: 10 * time.Second, // Default
		retries: 3,                // Default
		headers: make(map[string]string),
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func TestNewClient_FunctionalOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		baseURL     string
		opts        []Option
		wantTimeout time.Duration
		wantRetries int
		wantErr     bool
	}{
		{
			name:        "uses defaults when no options provided",
			baseURL:     "https://api.example.com",
			opts:        nil,
			wantTimeout: 10 * time.Second,
			wantRetries: 3,
			wantErr:     false,
		},
		{
			name:    "custom timeout and retries applied",
			baseURL: "https://api.example.com",
			opts: []Option{
				WithTimeout(5 * time.Second),
				WithRetries(5),
				WithHeader("Authorization", "Bearer token123"),
			},
			wantTimeout: 5 * time.Second,
			wantRetries: 5,
			wantErr:     false,
		},
		{
			name:    "error on invalid timeout",
			baseURL: "https://api.example.com",
			opts: []Option{
				WithTimeout(-1 * time.Second),
			},
			wantErr: true,
		},
		{
			name:    "error on negative retries",
			baseURL: "https://api.example.com",
			opts: []Option{
				WithRetries(-2),
			},
			wantErr: true,
		},
		{
			name:    "error on empty base URL",
			baseURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(tt.baseURL, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if client.timeout != tt.wantTimeout {
					t.Errorf("timeout = %v, want %v", client.timeout, tt.wantTimeout)
				}
				if client.retries != tt.wantRetries {
					t.Errorf("retries = %v, want %v", client.retries, tt.wantRetries)
				}
			}
		})
	}
}
