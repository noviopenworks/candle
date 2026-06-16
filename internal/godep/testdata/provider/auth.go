// Package auth provides token helpers.
package auth

// Client is an auth client.
type Client struct{}

// NewClient builds a Client.
func NewClient() *Client { return &Client{} }

// Verify checks a token.
func (c *Client) Verify(token string) bool { return token != "" }

// MaxRetries is the retry cap.
const MaxRetries = 3

func internalHelper() {}
