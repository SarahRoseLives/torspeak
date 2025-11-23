package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cretz/bine/tor"
)

// ANSI Color Codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m" // Peer color
	ColorCyan   = "\033[36m" // Self color
	ColorGray   = "\033[90m" // System messages
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	mode := os.Args[1]

	fmt.Printf("%sStarting Tor (this may take a moment)...%s\n", ColorGray, ColorReset)
	t, err := tor.Start(nil, nil)
	if err != nil {
		log.Panicf("Unable to start Tor: %v", err)
	}
	defer t.Close()

	switch mode {
	case "host":
		runHost(t)
	case "connect":
		if len(os.Args) < 3 {
			fmt.Println("Error: Missing onion address.")
			printUsage()
			return
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fmt.Printf("%sCreating Tor Hidden Service...%s\n", ColorGray, ColorReset)

	onion, err := t.Listen(ctx, &tor.ListenConf{
		Version3:    true,
		RemotePorts: []int{80},
	})
	if err != nil {
		log.Panicf("Unable to create onion service: %v", err)
	}
	defer onion.Close()

	fmt.Printf("\n%sCheck check. Secure line ready.%s\n", ColorGreen, ColorReset)
	fmt.Printf("COMMAND: torspeak connect %s.onion\n", onion.ID)
	fmt.Printf("\n%sWaiting for peer to connect...%s\n", ColorGray, ColorReset)

	conn, err := onion.Accept()
	if err != nil {
		log.Panicf("Accept failed: %v", err)
	}

	fmt.Printf("%s>> Peer connected! Start typing.%s\n", ColorGreen, ColorReset)
	stream(conn)
}

func runClient(t *tor.Tor, address string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	dialer, err := t.Dialer(ctx, nil)
	if err != nil {
		log.Panicf("Unable to create dialer: %v", err)
	}

	if !strings.Contains(address, ":") {
		address = address + ":80"
	}

	fmt.Printf("%sConnecting to %s...%s\n", ColorGray, address, ColorReset)

	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		log.Panicf("Failed to connect: %v", err)
	}

	fmt.Printf("%s>> Connected! Start typing.%s\n", ColorGreen, ColorReset)
	stream(conn)
}

func stream(conn net.Conn) {
	defer conn.Close()

	// Channel to signal when the chat is done
	done := make(chan struct{})

	// 1. INCOMING MESSAGES (Peer)
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			msg := scanner.Text()
			ts := time.Now().Format("15:04")
			// Format: [Time] <Peer>: Message (in Green)
			fmt.Printf("\r%s[%s] <Peer>: %s%s\n", ColorGreen, ts, msg, ColorReset)

			// Re-print the prompt cursor so typing looks clean
			fmt.Print("> ")
		}
		fmt.Printf("\n%s[Peer disconnected]%s\n", ColorRed, ColorReset)
		close(done)
	}()

	// 2. OUTGOING MESSAGES (Self)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("> ") // Initial prompt
		for scanner.Scan() {
			msg := scanner.Text()

			// Clear previous line to replace raw input with formatted input
			// \033[1A moves cursor up, \033[K clears line
			fmt.Printf("\033[1A\033[K")

			ts := time.Now().Format("15:04")

			// Format: [Time] <Me>: Message (in Cyan)
			fmt.Printf("%s[%s] <Me>: %s%s\n", ColorCyan, ts, msg, ColorReset)

			// Send raw message to peer (with newline)
			fmt.Fprintf(conn, "%s\n", msg)

			fmt.Print("> ") // Prompt for next line
		}
		// If stdin closes (Ctrl+D), close connection
		conn.Close()
	}()

	<-done
}