package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cretz/bine/tor"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	mode := os.Args[1]

	// 1. Start the Tor instance
	// This will download a static Tor binary if not found, or use the local one.
	// It takes a few seconds to bootstrap.
	fmt.Println("Starting Tor (this may take a moment)...")
	t, err := tor.Start(nil, nil)
	if err != nil {
		log.Panicf("Unable to start Tor: %v", err)
	}
	defer t.Close()

	// 2. Route based on command
	switch mode {
	case "host":
		runHost(t)
	case "connect":
		if len(os.Args) < 3 {
			fmt.Println("Error: Missing onion address.")
			printUsage()
			return
		}
		// Clean up address (add port 80 if missing, though we handle logic below)
		addr := os.Args[2]
		runClient(t, addr)
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  torspeak host                     # Start a chat server")
	fmt.Println("  torspeak connect <address.onion>  # Connect to a host")
}

func runHost(t *tor.Tor) {
	// Create a context for the listener setup
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Println("Creating Tor Hidden Service...")

	// Create an Onion V3 Hidden Service listening on virtual port 80
	onion, err := t.Listen(ctx, &tor.ListenConf{
		Version3:    true,
		RemotePorts: []int{80},
	})
	if err != nil {
		log.Panicf("Unable to create onion service: %v", err)
	}
	defer onion.Close()

	fmt.Printf("\nCheck check. Secure line ready.\n")
	fmt.Printf("COMMAND: torspeak connect %s.onion\n", onion.ID)
	fmt.Printf("\nWaiting for peer to connect...\n")

	// Accept the first incoming connection
	conn, err := onion.Accept()
	if err != nil {
		log.Panicf("Accept failed: %v", err)
	}

	fmt.Println(">> Peer connected! Start typing.")
	stream(conn)
}

func runClient(t *tor.Tor, address string) {
	// Create a context for the dialer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Create a Tor dialer
	dialer, err := t.Dialer(ctx, nil)
	if err != nil {
		log.Panicf("Unable to create dialer: %v", err)
	}

	// Ensure port 80 is attached to the string
	if !strings.Contains(address, ":") {
		address = address + ":80"
	}

	fmt.Printf("Connecting to %s...\n", address)

	// Connect via Tor
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		log.Panicf("Failed to connect: %v", err)
	}

	fmt.Println(">> Connected! Start typing.")
	stream(conn)
}

// stream pipes stdin -> conn and conn -> stdout
func stream(conn net.Conn) {
	defer conn.Close()

	// Channel to signal when the chat is done
	done := make(chan struct{})

	// Incoming: Read from socket, print to screen
	go func() {
		io.Copy(os.Stdout, conn)
		fmt.Println("\n[Peer disconnected]")
		close(done)
	}()

	// Outgoing: Read from keyboard, send to socket
	go func() {
		io.Copy(conn, os.Stdin)
		// If stdin closes (Ctrl+D), we close the connection
		conn.Close()
	}()

	// Block until the incoming stream ends
	<-done
}