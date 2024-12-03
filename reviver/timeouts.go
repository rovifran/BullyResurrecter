package main

import "time"

const (
	ElectionTimeout        = 5 * time.Second
	OkResponseTimeout      = 2 * time.Second
	PongTimeout            = 3 * time.Second
	PingToLeaderTimeout    = 1 * time.Second
	ResurrecterPingTimeout = 250 * time.Millisecond
	ResurrecterPingInterval = 1 * time.Second
	ResurrecterRestartDelay = 5 * time.Second
	ResurrecterPingRetries  = 4
)
