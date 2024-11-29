package shared

import (
	"encoding/gob"
	"fmt"
	"net"
)

type MessageType1 int

const (
	MessageTypePing MessageType1 = iota
	MessageTypePong MessageType1 = 1
)

	
func ListenUDP(port int) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func RunUDPListener(port int) error {
	conn, err := ListenUDP(port)
	if err != nil {
		return err
	}
	decoder := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)
	handleConnection(decoder, encoder)
	return nil
}

func handleConnection(decoder *gob.Decoder, encoder *gob.Encoder) {
	for {
		msg := new(ResurrecterMessage)
		err := decoder.Decode(msg)
		if err != nil {
			fmt.Printf("Error decoding message: %v\n", err)
			continue
		}
		switch msg.Message {
		case "ping":
			if err := encoder.Encode(ResurrecterMessage{Message: "pong", processName: msg.processName}); err != nil {
				fmt.Printf("Error encoding message: %v\n", err)
			}
		}
	}
}
