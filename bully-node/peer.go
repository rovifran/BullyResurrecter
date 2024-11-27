package main // Representa a un PEER al que YO llamo y consulto, cuando el me consulta a mi usa su pripio call, no es full duplex
//
// A -> B y B -> A
// en vez de A <--> B
import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
)

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
