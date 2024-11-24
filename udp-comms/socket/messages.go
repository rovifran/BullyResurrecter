package udp_comms

type TestMessage struct {
	Data string
}

type MessageInterface interface {
	TestMessage
}
