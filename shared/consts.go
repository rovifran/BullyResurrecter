package shared

import "time"

const (
	ElectionTimeout     = 5 * time.Second
	OkResponseTimeout   = 2 * time.Second
	PongTimeout         = 3 * time.Second
	PingToLeaderTimeout = 1 * time.Second
)
