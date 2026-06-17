package auth

// ValidateToken is the real exported symbol in the auth package.
func ValidateToken(token string) bool {
	return token != ""
}

// NewClient constructs an auth client.
func NewClient() *Client {
	return &Client{}
}

// Client is the auth client.
type Client struct{}
