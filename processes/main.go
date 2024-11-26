package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	messagePing = "ping\n" // Initial message
	messagePong = "pong\n" // Response to the message
)

func main() {
	cliId, err := strconv.Atoi(os.Getenv("CLI_ID"))
	if err != nil {
		fmt.Printf("Error getting CLI_ID: %v\n", err)
		os.Exit(1)
	}
	if cliId == 0 {
		fmt.Println("CLI_ID not specified")
		os.Exit(1)
	}
	bullyNodes, err := strconv.Atoi(os.Getenv("BULLY_NODES"))
	if err != nil {
		fmt.Printf("Error getting BULLY_NODES: %v\n", err)
		os.Exit(1)
	}

	node := NewNode(cliId)
	node.CreateTopology(bullyNodes)
	go node.Listen()

	time.Sleep(1 * time.Second)

	start := time.Now()
	for i := 0; i < 100; i++ {

		for _, peer := range node.peers {
			peer.Send(Message{PeerId: cliId, Type: MessageTypePing, Seq: i})

			response := new(Message)
			err := peer.decoder.Decode(response)
			if err != nil {
				fmt.Printf("Error decoding response: %v\n", err)
				continue
			}

			fmt.Printf("Received response from %s: %+v\n", peer.ip.String(), response)

			if rand.Int32N(100) < 3 && peer.conn != nil {
				log.Printf("Randomly closing peer %s\n", peer.ip.String())
				peer.Close()
			}
		}
	}

	log.Printf("Total time: %dms\n", time.Since(start).Milliseconds())

	select {}
}

type Node struct {
	id         int
	peers      []*Peer
	serverConn *net.TCPListener
	serverAddr *net.TCPAddr
	peerLock   sync.Mutex
}

func NewNode(id int) *Node {
	peers := []*Peer{}

	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("10.5.1.%d:8000", id))
	if err != nil {
		fmt.Printf("Error resolving server address: %v\n", err)
		os.Exit(1)
	}

	return &Node{id: id, peers: peers, serverAddr: serverAddr, peerLock: sync.Mutex{}}
}

func (n *Node) CreateTopology(bullyNodes int) {
	for i := 1; i <= bullyNodes; i++ {
		if i == n.id {
			continue
		}
		peerIp := fmt.Sprintf("10.5.1.%d", i)
		peer := NewPeer(&peerIp)
		if peer != nil {
			n.peers = append(n.peers, peer)
		}
	}
}

func (n *Node) Listen() {
	serverConn, err := net.ListenTCP("tcp", n.serverAddr)
	if err != nil {
		fmt.Printf("Error listening on server address: %v\n", err)
		os.Exit(1)
	}
	n.serverConn = serverConn

	for {

		conn, err := serverConn.AcceptTCP()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}

		go n.RespondToPeer(conn)
	}
}

func (n *Node) RespondToPeer(conn *net.TCPConn) {
	log.Printf("Peer %s connected\n", conn.RemoteAddr().String())

	read := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	for {
		message := new(Message)
		err := read.Decode(message)

		if err != nil {
			fmt.Printf("Error decoding message: %v\n", err)
			conn.Close()
			return
		}

		switch message.Type {
		case MessageTypePing:
			encoder.Encode(Message{PeerId: n.id, Type: MessageTypePong, Seq: message.Seq})
		case MessageTypePong:
		}
	}
}

type Peer struct {
	ip      *net.IP
	conn    *net.TCPConn
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func NewPeer(ip *string) *Peer {
	ipAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8000", *ip))
	if err != nil {
		fmt.Printf("Error resolving peer IP address: %v\n", err)
		return nil
	}

	return &Peer{ip: &ipAddr.IP, conn: nil, encoder: nil, decoder: nil}
}

func (p *Peer) call() error {
	log.Printf("Calling Peer %s...\n", p.ip.String())
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: *p.ip, Port: 8000})
	if err != nil {
		return err
	}
	p.conn = conn
	p.encoder = gob.NewEncoder(conn)
	p.decoder = gob.NewDecoder(conn)
	return nil
}

func (p *Peer) Close() {
	p.conn.Close()
	p.conn = nil
}

func (p *Peer) Send(message Message) error {
	if p.conn == nil {
		err := p.call()
		if err != nil {
			log.Printf("Error calling peer %s: %v\n", p.ip.String(), err)
			log.Printf("Restarting peer %s...\n", p.ip.String())
			return err
		}
	}

	err := p.encoder.Encode(message)
	if err != nil {
		log.Printf("Error sending message to %s: %v\n", p.ip.String(), err)
		return err
	}

	return nil
}

type MessageType int

const (
	MessageTypePing MessageType = iota
	MessageTypePong
)

type Message struct {
	PeerId int
	Type   MessageType
	Seq    int
}
