package udp_comms

// import (
// 	"bytes"
// 	"encoding/gob"
// 	"errors"
// 	"net"
// 	"time"
// )

// const (
// 	maxRetries   = 5
// 	timeoutDelay = 1 * time.Second
// )

// type Socket struct {
// 	conn        *net.UDPConn
// 	remoteAddr  *net.UDPAddr
// 	seqNum      uint16
// 	expectedSeq uint16
// }

// // Message represents our protocol message
// type Message struct {
// 	SeqNum uint16
// 	AckNum uint16
// 	Data   []byte
// }

// // Send sends data reliably using Stop and Wait
// func (s *Socket) Send(data []byte) error {
// 	msg := Message{
// 		SeqNum: s.seqNum,
// 		Data:   data,
// 	}

// 	encodedMsg, err := gobEncode(msg)
// 	if err != nil {
// 		return err
// 	}

// 	for attempts := 0; attempts < maxRetries; attempts++ {
// 		// Send the packet
// 		if _, err := s.conn.WriteToUDP(encodedMsg, s.remoteAddr); err != nil {
// 			return err
// 		}

// 		// Wait for ACK
// 		s.conn.SetReadDeadline(time.Now().Add(timeoutDelay))

// 		buffer := make([]byte, 1024)
// 		n, _, err := s.conn.ReadFromUDP(buffer)
// 		if err != nil {
// 			if errors.Is(err, net.ErrDeadlineExceeded) {
// 				continue // Timeout occurred, retry
// 			}
// 			return err
// 		}

// 		var ack Message
// 		if err := gobDecode(buffer[:n], &ack); err != nil {
// 			continue
// 		}

// 		if ack.AckNum == s.seqNum+1 {
// 			s.seqNum = (s.seqNum + 1) % 65536
// 			return nil
// 		}
// 	}

// 	return errors.New("max retries exceeded")
// }

// // Receive receives data and sends acknowledgment
// func (s *Socket) Receive(buffer []byte) (int, error) {
// 	for {
// 		// Receive data
// 		n, addr, err := s.conn.ReadFromUDP(buffer)
// 		if err != nil {
// 			return 0, err
// 		}

// 		var msg Message
// 		if err := gobDecode(buffer[:n], &msg); err != nil {
// 			continue
// 		}

// 		// Send ACK
// 		ack := Message{AckNum: msg.SeqNum + 1}
// 		encodedAck, err := gobEncode(ack)
// 		if err != nil {
// 			return 0, err
// 		}

// 		_, err = s.conn.WriteToUDP(encodedAck, addr)
// 		if err != nil {
// 			return 0, err
// 		}

// 		// Check if this is the packet we're expecting
// 		if msg.SeqNum == s.expectedSeq {
// 			s.expectedSeq = (s.expectedSeq + 1) % 65536
// 			// Copy received data to buffer
// 			copy(buffer, msg.Data)
// 			return len(msg.Data), nil
// 		}
// 	}
// }

// // Helper functions for GOB encoding/decoding
// func gobEncode(msg Message) ([]byte, error) {
// 	var buf bytes.Buffer
// 	enc := gob.NewEncoder(&buf)
// 	if err := enc.Encode(msg); err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }

// func gobDecode(data []byte, msg *Message) error {
// 	buf := bytes.NewBuffer(data)
// 	dec := gob.NewDecoder(buf)
// 	return dec.Decode(msg)
// }
