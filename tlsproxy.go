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

	destHosts := make(map[string]string)
	for _, serv := range flag.Args() {
		toAddr := serv
		if strings.Contains(serv, ":") {
			result := strings.SplitN(serv, ":", 2)
			serv, toAddr = result[0], result[1]
		}
		destHosts[serv] = toAddr
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
		go handleTlsConnection(destHosts, conn)
	}
}

func handleTlsConnection(destHosts map[string]string, inConn net.Conn) {
	var err error
	var serverName string

	defer inConn.Close()

	bufConn := &bufferConn{conn: inConn}
	{
		tlsConn := tls.Server(bufConn, nil)
		serverName, err = tlsConn.SniHandshake()
		if err != nil {
			log.Print(err)
			return
		}
	}

	destName, ok := destHosts[serverName]
	if !ok {
		log.Printf("connection requested to `%s' does not match any destination host", serverName)
		return
	}

	log.Printf("tls connection for %s, connecting to %s", serverName, destName)

	addr := destName
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:443", destName)
	}
	outConn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Print(err)
		return
	}
	defer outConn.Close()

	inCount, outCount := int64(len(bufConn.buf)), int64(0)
	_, err = outConn.Write(bufConn.buf)
	if err != nil {
		log.Print(err)
		return
	}

	c := make(chan bool)
	go func() {
		defer func() { c <- true }()
		count, err := io.Copy(outConn, inConn)
		if err != nil {
			log.Print(err)
		}
		inCount += count
	}()
	count, err := io.Copy(inConn, outConn)
	if err != nil {
		log.Print(err)
	}
	outCount += count

	<-c
	log.Printf("tls connection closed, in: %d, out: %d", inCount, outCount)
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
