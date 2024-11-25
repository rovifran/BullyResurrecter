package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	messagePing = "ping" // Initial message
	messagePong = "pong" // Response to the message
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
	listenPort, err := strconv.Atoi(os.Getenv("LISTEN_PORT"))
	if err != nil {
		fmt.Printf("Error getting LISTEN_PORT: %v\n", err)
		os.Exit(1)
	}

	node := NewNode(cliId, listenPort)
	go node.Listen()

	time.Sleep(1 * time.Second)

	node.ConnectToPeers(bullyNodes, listenPort)

	select {}
}

type Node struct {
	id         int
	peers      []*Peer
	serverConn *net.TCPListener
	serverAddr *net.TCPAddr
	wg         sync.WaitGroup
}

func NewNode(id int, listenPort int) *Node {
	peers := []*Peer{}

	serverAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		fmt.Printf("Error resolving server address: %v\n", err)
		os.Exit(1)
	}

	return &Node{id: id, peers: peers, serverAddr: serverAddr, wg: sync.WaitGroup{}}
}

func (n *Node) ConnectToPeers(bullyNodes int, listenPort int) {
	for i := 1; i <= bullyNodes; i++ {
		if i == n.id {
			continue
		}
		peerAddr := fmt.Sprintf("10.5.1.%d:%d", i, listenPort)
		n.wg.Wait()
		n.wg.Add(1)
		peer := CallPeer(i, peerAddr)
		if peer != nil {
			n.peers = append(n.peers, peer)
			log.Printf("Called Peer %d correctly\n", i)
		}
		n.wg.Done()
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

			log.Printf("Accepted connection from %s\n", conn.RemoteAddr().String())

			n.wg.Wait()
			n.wg.Add(1)
			defer n.wg.Done()

			// check if the peer is already in the list
			addr := conn.RemoteAddr().(*net.TCPAddr)
			for _, peer := range n.peers {
				if peer.ip.String() == addr.IP.String() {
					log.Printf("Peer %s was already connected, closing old connection...\n", peer.ip.String())
					peer.conn.Close()
					peer.conn = conn
					// TODO: reiniciar loop del Peer con la nueva conexion
					return
				}
			}

			peer := AcceptPeer(conn)
			if peer != nil {
				n.peers = append(n.peers, peer)
				// log.Printf("New peer connected from %s\n", conn.LocalAddr())
			}
		}()
	}

}

type Peer struct {
	ip   *net.IP
	conn *net.TCPConn
}

func CallPeer(id int, addrString string) *Peer {

	addr, err := net.ResolveTCPAddr("tcp", addrString)
	if err != nil {
		fmt.Printf("Error resolving peer address: %v\n", err)
		return nil
	}

	peer := &Peer{ip: &addr.IP}

	if err := peer.connect(); err != nil {
		fmt.Printf("Error connecting to peer: %v\n", err)
		return nil
	}

	go peer.Listen()

	return peer
}

func AcceptPeer(conn *net.TCPConn) *Peer {
	addr := conn.LocalAddr().(*net.TCPAddr)

	peer := &Peer{conn: conn, ip: &addr.IP}

	go peer.Listen()

	return peer
}

func (p *Peer) connect() error {
	log.Printf("Calling Peer at %s\n", p.ip.String())
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: *p.ip, Port: 8000})
	if err != nil {
		return err
	}
	p.conn = conn
	return nil
}

func (p *Peer) Close() error {
	return p.conn.Close()
}

func (p *Peer) Send(message string) error {
	_, err := p.conn.Write([]byte(message))
	return err
}

func (p *Peer) Listen() {
	buffer := make([]byte, 1024)
	for {
		n, err := p.conn.Read(buffer)
		if err != nil {
			fmt.Printf("Error reading from peer: %v\n", err)
			return
		}

		received := string(buffer[:n])
		fmt.Printf("Received: %s\n", received)
	}
}
