package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http/httputil"
	"strings"
	"time"

	"./contrib/tls"
)

var destHosts map[string]string

func main() {
	var tlsPort = flag.String("tlsport", "https", "tls port to listen on")
	var httpPort = flag.String("httpport", "", "http port to listen on")
	flag.Parse()

	if *tlsPort != "" && !strings.Contains(*tlsPort, ":") {
		*tlsPort = fmt.Sprintf(":%s", *tlsPort)
	}
	if *httpPort != "" && !strings.Contains(*httpPort, ":") {
		*httpPort = fmt.Sprintf(":%s", *httpPort)
	}

	destHosts = make(map[string]string)
	for _, serv := range flag.Args() {
		toAddr := serv
		if strings.Contains(serv, ":") {
			result := strings.SplitN(serv, ":", 2)
			serv, toAddr = result[0], result[1]
		}
		destHosts[serv] = toAddr
	}

	if *httpPort != "" {
		httpListen, err := net.Listen("tcp", *httpPort)
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			for {
				conn, err := httpListen.Accept()
				if err != nil {
					log.Print(err)
					continue
				}
				go handleHttpConnection(conn)
			}
		}()
	}

	tlsListen, err := net.Listen("tcp", *tlsPort)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := tlsListen.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleTlsConnection(conn)
	}
}

func handleTlsConnection(inConn net.Conn) {
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

	proxyConn(bufConn, serverName, "https")
}

func handleHttpConnection(inConn net.Conn) {
	var serverName string

	defer inConn.Close()

	bufConn := &bufferConn{conn: inConn}
	{
		httpConn := httputil.NewServerConn(bufConn, nil)
		req, err := httpConn.Read()
		if err != nil {
			log.Print(err)
			return
		}
		serverName = req.Host
	}

	proxyConn(bufConn, serverName, "http")
}

func proxyConn(bufConn *bufferConn, host string, port string) {
	destName, ok := destHosts[host]
	if !ok {
		log.Printf("connection requested to `%s' does not match any destination host", host)
		return
	}

	log.Printf("connection for %s, connecting to %s", host, destName)

	addr := destName
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%s", destName, port)
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
		count, err := io.Copy(outConn, bufConn.conn)
		if err != nil {
			log.Print(err)
		}
		inCount += count
	}()
	count, err := io.Copy(bufConn.conn, outConn)
	if err != nil {
		log.Print(err)
	}
	outCount += count

	<-c
	log.Printf("connection closed, in: %d, out: %d", inCount, outCount)
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
