package discord

import "fmt"

// Error constants
var (
	ErrInvalidToken = fmt.Errorf("discord connector requires a valid token")
)
