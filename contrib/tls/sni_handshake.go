// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

func (c *Conn) SniHandshake() (serverName string, err error) {
	msg, err := c.readHandshake()
	if err != nil {
		return "", err
	}
	clientHello, ok := msg.(*clientHelloMsg)
	if !ok {
		return "", unexpectedMessageError(clientHello, msg)
	}
	return clientHello.serverName, nil
}
