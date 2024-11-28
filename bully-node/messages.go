package main

type MessageType int

const (
	MessageTypePing        MessageType = iota
	MessageTypePong        MessageType = 1
	MessageTypeElection    MessageType = 2
	MessageTypeCoordinator MessageType = 3
	MessageTypeOk          MessageType = 4
)

type Message struct {
	PeerId int
	Type   MessageType
}
