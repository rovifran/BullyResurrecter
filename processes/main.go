package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	cliId, err := strconv.Atoi(os.Getenv("CLI_ID"))
	if err != nil {
		log.Printf("Error getting CLI_ID: %v\n", err)
		os.Exit(1)
	}
	if cliId == 0 {
		fmt.Println("CLI_ID not specified")
		os.Exit(1)
	}
	bullyNodes, err := strconv.Atoi(os.Getenv("BULLY_NODES"))
	if err != nil {
		log.Printf("Error getting BULLY_NODES: %v\n", err)
		os.Exit(1)
	}

	node := NewNode(cliId)
	go node.Listen()

	node.CreateTopology(bullyNodes)

	time.Sleep(1 * time.Second)

	for i := 0; i < 1000; i++ {
		for _, peer := range node.peers {
			peer.SendMsg(Message{Type: MessageTypePing, Data: "Hello, world!"})
		}
	}

	time.Sleep(1 * time.Hour)
}

type MessageType int32

const (
	MessageTypePing MessageType = iota
	MessageTypeAck
)

type Message struct {
	Type   MessageType
	SeqNum uint16
	Data   string
}

type Node struct {
	id         int
	peers      []*Peer
	serverConn *net.UDPConn
	serverAddr *net.UDPAddr
}

func NewNode(id int) *Node {
	peers := []*Peer{}

	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("10.5.1.%d:8000", id))
	if err != nil {
		log.Printf("Error resolving server address: %v\n", err)
		os.Exit(1)
	}

	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		log.Printf("Error listening on server address: %v\n", err)
		os.Exit(1)
	}

	return &Node{id: id, peers: peers, serverAddr: serverAddr, serverConn: serverConn}
}

func (n *Node) CreateTopology(bullyNodes int) {
	for i := 1; i <= bullyNodes; i++ {
		if i == n.id {
			continue
		}
		peerIp := fmt.Sprintf("10.5.1.%d", i)
		peer := NewPeer(&peerIp, n.serverConn)
		if peer != nil {
			n.peers = append(n.peers, peer)
		}
	}
}

func (n *Node) Listen() {

	for {
		func() {
			var msg Message

			innerBuffer := make([]byte, 1024)

			_, who, err := n.serverConn.ReadFromUDP(innerBuffer)
			if err != nil {
				log.Printf("Error reading from server connection: %v\n", err)
				os.Exit(1)
			}

			decoder := gob.NewDecoder(bytes.NewReader(innerBuffer))
			if err := decoder.Decode(&msg); err != nil {
				log.Printf("Error decoding message: %v\n", err)
				return
			}

			for _, peer := range n.peers {
				if peer.addr.String() == who.String() {
					peer.in <- msg
				}
			}
		}()
	}
}

type Peer struct {
	addr       *net.UDPAddr
	conn       *net.UDPConn
	out        chan Message
	in         chan Message
	pendingOut *Message
}

func NewPeer(ip *string, conn *net.UDPConn) *Peer {
	ipAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:8000", *ip))
	if err != nil {
		log.Printf("Error resolving peer IP address: %v\n", err)
		return nil
	}

	peer := Peer{addr: ipAddr, conn: conn, out: make(chan Message, 100), in: make(chan Message, 100)}
	go peer.InLoop()
	return &peer
}

func (p *Peer) InLoop() {
	for msg := range p.in {
		if msg.Type == MessageTypeAck {
			log.Printf("<<< ACK %s: %d", p.addr.String(), msg.SeqNum)
		} else {
			log.Printf("<<< %s: %d", p.addr.String(), msg.SeqNum)
		}
		switch msg.Type {
		case MessageTypePing:
			p.Ack(msg.SeqNum)
		case MessageTypeAck:
			if p.pendingOut != nil && p.pendingOut.SeqNum == msg.SeqNum {
				p.pendingOut = nil
				p._send(msg.SeqNum + 1)
			} else if p.pendingOut != nil {
				log.Printf("Ack for invalid pending out, waiting for %d and got %d", p.pendingOut.SeqNum, msg.SeqNum)
			}
		default:
			log.Printf("Unknown message type: %d", msg.Type)
		}

	}

	log.Printf("<<< IN LOOP ENDED")
}

func (p *Peer) SendMsg(msg Message) error {
	empty := len(p.out) == 0
	p.out <- msg
	if empty {
		p._send(0)
	}
	return nil
}

func (p *Peer) _send(seqNum uint16) error {
	if p.pendingOut != nil {
		return nil
	}
	log.Printf(">>> %s: %d", p.addr.String(), seqNum)

	msg := <-p.out
	msg.SeqNum = seqNum
	p.pendingOut = &msg

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		return err
	}

	_, err := p.conn.WriteToUDP(buf.Bytes(), p.addr)
	if err != nil {
		log.Printf("Error sending message: %v\n", err)
	}
	return err
}

func (p *Peer) Ack(seqNum uint16) {
	log.Printf(">>> ACK %s: %d", p.addr.String(), seqNum)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(Message{Type: MessageTypeAck, SeqNum: seqNum}); err != nil {
		log.Printf("Error encoding ack: %v\n", err)
		return
	}

	_, err := p.conn.WriteToUDP(buf.Bytes(), p.addr)
	if err != nil {
		log.Printf("Error sending ack: %v\n", err)
	}
}
