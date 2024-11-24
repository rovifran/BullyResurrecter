package udp_comms

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("DEBUG")

const MAX_TIMEOUT = time.Duration(math.MaxInt64)
const MAX_RETRIES = 5

// const (
// 	maxRetries   = 5
// 	timeoutDelay = 1 * time.Second
// )

// Message represents the message that is sent over the network
type Message struct {
	IsAck  bool
	SeqNum uint16
	Dest   string // process name
	Data   []byte
}

// PendingMessage represents a message that is waiting to be acknowledged
type PendingMessage struct {
	Msg      Message
	Retries  uint8
	SentAt   time.Time
	Timeout  time.Duration
	Callback func(Message)
}

// PeerInfo holds the information about a peer, this is, its address and the
// sequence number of the last message it has sent
type PeerInfo struct {
	Addr   *net.UDPAddr
	SeqNum uint16
}

// Socket is the struct that represents an UDP socket
// that implements Stop and Wait protocol for reliable communication
type Socket struct {
	conn            *net.UDPConn
	pendingMessages map[string]map[uint16]PendingMessage
	processName     string
	localAddr       *net.UDPAddr
	peers           map[string]*PeerInfo
}

// NewSocket creates a new socket and initializes the addresses
// for its corresponding peer processes
func NewSocket(processName string, peers []string, port int) (*Socket, error) {
	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", processName, port))
	if err != nil {
		return nil, fmt.Errorf("error resolving local address: %v", err)
	}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("error listening on UDP port: %v", err)
	}

	peersInfo := make(map[string]*PeerInfo)
	for _, peer := range peers {
		peerAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer, port))
		if err != nil {
			return nil, fmt.Errorf("error resolving peer address: %v", err)
		}
		peersInfo[peer] = &PeerInfo{Addr: peerAddr, SeqNum: 0}
	}

	return &Socket{
		conn:            conn,
		pendingMessages: make(map[string]map[uint16]PendingMessage),
		processName:     processName,
		localAddr:       localAddr,
		peers:           peersInfo,
	}, nil
}

// Close closes the socket
func (s *Socket) Close() error {
	return s.conn.Close()
}

// _send sends a message to a destination
func (s *Socket) _send(msg Message, dest string, callback func(Message), retries uint8, timeout time.Duration) error {
	encodedMsg, err := gobEncode(msg)
	if err != nil {
		return fmt.Errorf("error encoding message: %v", err)
	}

	writtenBytes := 0
	for writtenBytes != len(encodedMsg) {
		n, err := s.conn.WriteToUDP(encodedMsg[writtenBytes:], s.peers[dest].Addr)
		if err != nil && err != net.ErrClosed {
			log.Infof("error sending message to peer %s with seqNum %d. Expected to write %d bytes, wrote %d bytes. Error: %v", dest, msg.SeqNum, len(encodedMsg), writtenBytes, err)
		}
		if err == net.ErrClosed {
			return fmt.Errorf("socket closed")
		}
		writtenBytes = n
	}

	s.pendingMessages[dest][msg.SeqNum] = PendingMessage{
		Msg:      msg,
		Retries:  retries,
		SentAt:   time.Now(),
		Timeout:  timeout,
		Callback: callback,
	}

	return nil
}

// Send sends a message to a destination
func (s *Socket) Send(data []byte, dest string, callback func(Message)) error {
	seqNum := s.peers[dest].SeqNum
	s.peers[dest].SeqNum = s.peers[dest].SeqNum + 1

	msg := Message{
		IsAck:  false,
		SeqNum: seqNum,
		Dest:   dest,
		Data:   data,
	}

	return s._send(msg, dest, callback, 0, 1*time.Second) // TODO estos valores de timeout y retries estan hardcodeados, cambiarlos despues por los que vienen x config
}

// Receive receives a message from a source. Handles the corresponding acks and timeouts.
// Returns an error if there were any critical ones or does not return anything if the message was received correctly,
// in which case the message is copied into the buffer
func (s *Socket) Receive(buffer []byte) error {
	for {
		innerBuffer := make([]byte, len(buffer))
		pendingMessagesWithSmallestTimeout := s.determineSmallestTimeout()
		if len(pendingMessagesWithSmallestTimeout) == 0 {
			if err := s.conn.SetReadDeadline(time.Time{}); err != nil {
				return fmt.Errorf("error setting read deadline: %v", err)
			}
		} else {
			if err := s.conn.SetReadDeadline(time.Now().Add(pendingMessagesWithSmallestTimeout[0].Timeout)); err != nil {
				return fmt.Errorf("error setting read deadline: %v", err)
			}
		}

		n, addr, err := s.conn.ReadFromUDP(innerBuffer)

		if err == net.ErrClosed {
			return fmt.Errorf("socket closed")
		}

		if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			return fmt.Errorf("error reading from UDP: %v", err)
		}

		s.recalculateTimeouts()

		if errors.Is(err, os.ErrDeadlineExceeded) {
			s.sendTimeoutMessages()
			continue
		}

		var msg Message
		if err := gobDecode(innerBuffer[:n], &msg); err != nil {
			return fmt.Errorf("error decoding message: %v", err)
		}

		if msg.IsAck {
			s.handleAck(msg)
		} else {
			if err := s.sendAck(msg, addr); err != nil {
				return fmt.Errorf("error sending ack: %v", err)
			}
			copy(buffer, msg.Data)
			return nil
		}
	}
}

// Helper functions for GOB encoding/decoding
func gobEncode(msg Message) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gobDecode(data []byte, msg *Message) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(msg)
}

// determineSmallestTimeout returns the pending messages with the smallest timeout
func (s *Socket) determineSmallestTimeout() []PendingMessage {
	smallestTimeout := time.Duration(math.MaxInt64)
	pendingMessagesResult := make([]PendingMessage, 0)
	for _, pendingMessages := range s.pendingMessages {
		for _, pendingMessage := range pendingMessages {
			if pendingMessage.Timeout < smallestTimeout {
				pendingMessagesResult = make([]PendingMessage, 0)
				smallestTimeout = pendingMessage.Timeout
			} else if pendingMessage.Timeout == smallestTimeout {
				pendingMessagesResult = append(pendingMessagesResult, pendingMessage)
			}
		}
	}
	return pendingMessagesResult
}

// recalculateTimeouts recalculates the timeouts of the pending messages
func (s *Socket) recalculateTimeouts() {
	now := time.Now()
	for _, processMessages := range s.pendingMessages {
		for _, pendingMessage := range processMessages {
			pendingMessage.Timeout = pendingMessage.Timeout - now.Sub(pendingMessage.SentAt)
		}
	}
}

func (s *Socket) handleAck(msg Message) {
	_, ok := s.pendingMessages[msg.Dest][msg.SeqNum]
	if !ok {
		return
	}
	delete(s.pendingMessages[msg.Dest], msg.SeqNum)
}

// sendAck sends an ack to a source
func (s *Socket) sendAck(msg Message, addr *net.UDPAddr) error {
	ack := Message{
		IsAck:  true,
		SeqNum: msg.SeqNum,
	}
	encodedAck, err := gobEncode(ack)
	if err != nil {
		return fmt.Errorf("error encoding ack: %v", err)
	}
	_, err = s.conn.WriteToUDP(encodedAck, addr)
	if err != nil {
		return fmt.Errorf("error sending ack: %v", err)
	}
	return nil
}

// sendTimeoutMessages sends the timeout messages to the corresponding destinations
func (s *Socket) sendTimeoutMessages() {
	for _, processMessages := range s.pendingMessages {
		for _, pendingMessage := range processMessages {
			if pendingMessage.Retries >= MAX_RETRIES {
				delete(s.pendingMessages[pendingMessage.Msg.Dest], pendingMessage.Msg.SeqNum)
				pendingMessage.Callback(pendingMessage.Msg)
			} else if pendingMessage.Timeout <= 0 {
				pendingMessage.Retries = pendingMessage.Retries + 1
				pendingMessage.Timeout = pendingMessage.Timeout * 2
				s._send(pendingMessage.Msg, pendingMessage.Msg.Dest, pendingMessage.Callback, pendingMessage.Retries, pendingMessage.Timeout)
			}
		}
	}
}
