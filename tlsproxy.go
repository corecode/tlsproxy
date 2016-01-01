package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"./contrib/tls"
)

func main() {
	var tlsport = flag.String("tlsport", "https", "tls port to listen on")
	flag.Parse()

	if !strings.Contains(*tlsport, ":") {
		*tlsport = fmt.Sprintf(":%s", *tlsport)
	}

	ln, err := net.Listen("tcp", *tlsport)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleTlsConnection(conn)
	}
}

func handleTlsConnection(inConn net.Conn) {
	bufConn := &bufferConn{conn: inConn}
	defer bufConn.Close()
	tlsConn := tls.Server(bufConn, nil)
	serverName, err := tlsConn.SniHandshake()
	if err != nil {
		log.Print(err)
		return
	}
	log.Printf("tls connection for %s, buffered %d", serverName, len(bufConn.buf))
	addr := fmt.Sprintf("%s:443", serverName)
	outConn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Print(err)
		return
	}
	defer outConn.Close()

	_, err = outConn.Write(bufConn.buf)
	if err != nil {
		log.Print(err)
		return
	}

	c := make(chan bool)
	go func() {
		defer func() { c <- true }()
		io.Copy(outConn, inConn)
	}()
	io.Copy(inConn, outConn)
	<-c

	log.Print("tls connection closed")
}

type bufferConn struct {
	conn net.Conn
	buf  []byte
}

func (c *bufferConn) Read(b []byte) (n int, err error) {
	n, err = c.conn.Read(b)
	c.buf = append(c.buf, b[0:n]...)
	return
}

func (c *bufferConn) Write(b []byte) (n int, err error) {
	// We don't support writing
	return 0, fmt.Errorf("bufferConn is not writable")
}

func (c *bufferConn) Close() error {
	return c.conn.Close()
}

func (c *bufferConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *bufferConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *bufferConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *bufferConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *bufferConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
