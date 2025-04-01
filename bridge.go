package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
)

var tcpPort int64
var serialPort string

func init() {
	flag.Int64Var(&tcpPort, "tcp", 0, "TCP port to expose to remote clients")
	flag.StringVar(&serialPort, "serial", "", "Serial (USB) port to read from")
}

func main() {
	flag.Parse()
	if tcpPort < 1 || serialPort == "" {
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		cancel()
	}()

	clients := make(chan net.Conn)
	go startServer(ctx, tcpPort, clients)

	lines := make(chan []byte)
	go readSerial(ctx, serialPort, lines)

	clientMap := make(map[net.Conn]bool)
	for {
		select {
		case line := <-lines:
			for conn := range clientMap {
				if _, err := conn.Write(line); err != nil {
					log.Printf("Error writing to client %v: %v", conn.RemoteAddr(), err)
					_ = conn.Close()
					delete(clientMap, conn)
				}
			}
		case conn := <-clients:
			clientMap[conn] = true
		case <-ctx.Done():
			log.Println("Closing all remote connections...")
			for client := range clientMap {
				if err := client.Close(); err != nil {
					log.Printf("Error closing client connection %v: %v", client.RemoteAddr(), err)
				}
			}
			return
		}
	}
}

func readSerial(ctx context.Context, serialPort string, lines chan []byte) {
	// Open the serial port
	f, err := openSerial(serialPort)
	if err != nil {
		log.Fatalf("Failed to open serial port: %v", err)
	}
	defer f.Close()

	go func() {
		<-ctx.Done()
		_ = f.Close()
	}()

	// Read data from the serial port
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n') // Read until newline
		if err != nil {
			log.Printf("Error reading from serial port: %v", err)
			return
		}
		lines <- line
	}
}

func startServer(ctx context.Context, port int64, clients chan net.Conn) {
	log.Printf("Listening on TCP port %d", port)
	server, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down server...")
		_ = server.Close()
	}()

	for {
		c, err := server.Accept()
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("Error accepting %v: %v", c.RemoteAddr(), err)
				_ = c.Close()
				continue
			} else {
				// ctx was cancelled
				break
			}
		}
		clients <- c
		log.Printf("Connected: %v", c.RemoteAddr())
	}
}
