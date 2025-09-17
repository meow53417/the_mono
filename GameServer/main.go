package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
)

func main() {
	addr := "127.0.0.1:8080"

	// Start TCP listener
	tcpLn, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("tcp listen: %v", err)
	}
	defer tcpLn.Close()

	// Start UDP listener
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		log.Fatalf("udp resolve: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("udp listen: %v", err)
	}
	defer udpConn.Close()

	// Connected TCP clients
	var (
		mu      sync.Mutex
		clients = map[net.Conn]struct{}{}
	)

	// Broadcast helper
	broadcast := func(msg []byte) {
		mu.Lock()
		defer mu.Unlock()
		for c := range clients {
			if _, err := c.Write(msg); err != nil {
				// remove dead client
				_ = c.Close()
				delete(clients, c)
			}
		}
	}

	// Accept TCP clients
	go func() {
		for {
			conn, err := tcpLn.Accept()
			if err != nil {
				log.Printf("accept: %v", err)
				continue
			}
			log.Printf("TCP client connected: %s", conn.RemoteAddr())

			mu.Lock()
			clients[conn] = struct{}{}
			mu.Unlock()

			// Optional: read from client in background (to detect disconnects)
			go func(c net.Conn) {
				defer func() {
					mu.Lock()
					delete(clients, c)
					mu.Unlock()
					_ = c.Close()
					log.Printf("TCP client disconnected: %s", c.RemoteAddr())
				}()
				r := bufio.NewReader(c)
				for {
					if _, err := r.ReadByte(); err != nil {
						return
					}
					// You can parse client input here if needed.
				}
			}(conn)
		}
	}()

	var state int64

	// Read UDP packets and broadcast to TCP clients
	buf := make([]byte, 65535)
	for {
		n, raddr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("udp read: %v", err)
			continue
		}

		msg := buf[:n]

		if val, err := strconv.ParseInt(string(msg), 10, 64); err == nil {
			state += val
		}

		log.Printf("UDP from %s: %q", raddr.String(), string(msg), "state: ", strconv.FormatInt(state, 10))

		// Append newline so TCP clients reading by lines can handle it
		out := append([]byte(fmt.Sprintf("[from %s] ", raddr.String(), "state: ", strconv.FormatInt(state, 10))), msg...)
		out = append(out, '\n')
		broadcast(out)
	}
}