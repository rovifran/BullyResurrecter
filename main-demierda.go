package main

import (
	"context"
	"errors"
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

	for i := 0; i < 100; i++ {

		for _, peer := range node.peers {
			err := peer.Send(fmt.Sprintf("Peer: %d, Message: %d\n", cliId, i))
			if err != nil {
				log.Printf("Error sending message to %s: %v\n", peer.ip.String(), err)
			}

			if rand.Int32N(100) < 3 && peer.conn != nil {
				log.Printf("Randomly closing peer %s\n", peer.ip.String())
				peer.Close()
			}
		}

		time.Sleep(1 * time.Second)
	}

	select {}
}

type Node struct {
	id         int
	peers      []*Peer
	serverConn *net.TCPListener
	serverAddr *net.TCPAddr
}

func NewNode(id int) *Node {
	peers := []*Peer{}

	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("10.5.1.%d:8000", id))
	if err != nil {
		fmt.Printf("Error resolving server address: %v\n", err)
		os.Exit(1)
	}

	return &Node{id: id, peers: peers, serverAddr: serverAddr}
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
		func() {

			conn, err := serverConn.AcceptTCP()

			if err != nil {
				fmt.Printf("Error accepting connection: %v\n", err)
				return
			}

			addr := conn.RemoteAddr().(*net.TCPAddr)
			for _, peer := range n.peers {
				if peer.ip.String() == addr.IP.String() {
					peer.accept(conn)
					return
				}
			}

			log.Printf("Unkown peer %s connected, closing connection...\n", conn.RemoteAddr().String())
			conn.Close()
		}()
	}
}

type Peer struct {
	ip        *net.IP
	conn      *net.TCPConn
	connMutex sync.Mutex
	iMutex    sync.Mutex
	i         int
}

func NewPeer(ip *string) *Peer {
	ipAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8000", *ip))
	if err != nil {
		fmt.Printf("Error resolving peer IP address: %v\n", err)
		return nil
	}

	return &Peer{ip: &ipAddr.IP, conn: nil, i: 0}
}

func (p *Peer) accept(conn *net.TCPConn) {
	p.connMutex.Lock()
	log.Printf("Accepting connection from %s\n", p.ip.String())

	if p.conn != nil {
		log.Printf("Peer %s was already connected, closing old connection...\n", p.ip.String())
		p.conn.Close()
	}

	p.conn = conn
	go p.Listen()
	p.connMutex.Unlock()
}

func (p *Peer) call() error {
	p.connMutex.Lock()
	log.Printf("Calling Peer %s...\n", p.ip.String())
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: *p.ip, Port: 8000})
	if p.conn != nil {
		log.Printf("Peer %s was already connected, closing old connection...\n", p.ip.String())
		p.conn.Close()
	}
	if err != nil {
		return err
	}
	p.conn = conn
	go p.Listen()
	p.connMutex.Unlock()
	return nil
}

func (p *Peer) SendMsg(message string) error {
	return p.Send(message)
}

func (p *Peer) ReceiveMsg() (message string, err error) {
	return "", nil
}

func (p *Peer) HeartBeat(ctx context.Context) {
	for {
		err := func() error {

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}

			for i := 0; i < 3; i++ {
				p.Send(messagePing)
				msg, err := p.ReceivePong()
				if err != nil {
					return err
				}

			}
			return errors.New("failed to receive Pong")
		}()

		if err != nil && err == ctx.Err() {
			return
		}

		if err != nil {
			Election()
		}
	}
}

func (p *Peer) ReceivePong() (message string, err error) {
	return "", nil
}

func (p *Peer) Close() {
	p.conn.Close()
	p.conn = nil
}

func (p *Peer) Send(message string) error {
	if p.conn == nil {
		err := p.call()
		if err != nil {
			log.Printf("Error calling peer %s: %v\n", p.ip.String(), err)
			log.Printf("Restarting peer %s...\n", p.ip.String())
			return err
		}
	}
	_, err := p.conn.Write([]byte(message))
	return err
}

func (p *Peer) Listen() {
	log.Printf("Listening for messages from %s\n", p.ip.String())
	buffer := make([]byte, 1024)
	for {
		n, err := p.conn.Read(buffer)
		if err != nil {
			break
		}

		received := string(buffer[:n])
		fmt.Printf("Received %d: %s", p.i, received)

		p.iMutex.Lock()
		p.i++
		p.iMutex.Unlock()
	}

	log.Printf("Disconnected from %s\n", p.ip.String())
}
