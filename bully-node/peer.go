package main // Representa a un PEER al que YO llamo y consulto, cuando el me consulta a mi usa su pripio call, no es full duplex
//
// A -> B y B -> A
// en vez de A <--> B
import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net"
	"syscall"
)

type Peer struct {
	id      int
	name    *string
	ip      *net.IP
	conn    *net.TCPConn
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func NewPeer(id int, name *string) *Peer {
	ipAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8000", *name))
	if err != nil {
		fmt.Printf("Error resolving peer IP address: %v\n", err)
		return nil
	}

	return &Peer{id: id, name: name, ip: &ipAddr.IP, conn: nil, encoder: nil, decoder: nil}
}

func (p *Peer) call() error {
	connAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:8000", *p.name))
	if err != nil {
		log.Printf("Error resolving peer %s: %v\n", p.ip.String(), err)
		return err
	}

	log.Printf("Calling Peer %s...\n", connAddr.String())
	conn, err := net.DialTCP("tcp", nil, connAddr)
	if err != nil {
		log.Printf("Error dialing peer %s: %v\n", p.ip.String(), err)
		return err
	}
	log.Printf("Connected to peer %s\n", connAddr.String())
	p.conn = conn
	p.encoder = gob.NewEncoder(conn)
	p.decoder = gob.NewDecoder(conn)
	return nil
}

func (p *Peer) Close() {
	log.Printf("Closing connection to %s\n", p.ip.String())
	p.conn.Close()
	p.conn = nil
}

func (p *Peer) Send(message Message) error {
	defer func() {
		recover() // a veces la falopa de message es nil pero tampoco nos deja comparar con nil porque Message es un tipo y no un struct opasniosafnoiasfinoas
	}()

	log.Printf("Sending message to %s: %v\n", p.ip.String(), message)
	if p.conn == nil {
		err := p.call()
		if err != nil {
			log.Printf("Error calling peer %s: %v\n", p.ip.String(), err)
			return err
		}
	}

	err := p.encoder.Encode(message)
	if err != nil && errors.Is(err, syscall.EPIPE) {
		// log.Printf("Peer %s disconnected: %v\n", p.ip.String(), err)
		p.Close()
		return err
	}

	if err != nil {
		log.Printf("Error sending message to %s: %v\n", p.ip.String(), err)
		p.Close()
		return err
	}

	return nil
}
