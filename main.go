package main

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cretz/bine/tor"
)

// ANSI Color Codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorCyan   = "\033[36m"
	ColorYellow = "\033[33m"
	ColorGray   = "\033[90m"
)

func main() {
	// 1. Setup Signal Interception (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if len(os.Args) < 2 {
		printUsage()
		return
	}
	mode := os.Args[1]

	// 2. Create a strictly ephemeral data directory
	dataDir, err := os.MkdirTemp("", "torspeak-session-*")
	if err != nil {
		log.Panicf("Failed to create temp dir: %v", err)
	}

	// 3. Define the Cleanup Function
	cleanup := func() {
		fmt.Printf("\n%s[!] Wiping session data and keys...%s", ColorRed, ColorReset)
		os.RemoveAll(dataDir)
		fmt.Println(" Done.")
		os.Exit(0)
	}

	defer cleanup()
	go func() {
		<-sigChan
		cleanup()
	}()

	// 4. Start Tor with the Custom Data Directory
	fmt.Printf("%sStarting Tor (initializing secure circuit)...%s\n", ColorGray, ColorReset)

	// FIX: We removed NoAutoSocksPort: true
	// This lets bine find a random free port (e.g., 54321) instead of crashing on 9050
	t, err := tor.Start(nil, &tor.StartConf{
		DataDir: dataDir,
	})
	if err != nil {
		os.RemoveAll(dataDir)
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
	fmt.Println("  torspeak host                     # Start a secure chat server")
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

	fmt.Printf("\n%sSECURE LINE READY.%s\n", ColorGreen, ColorReset)
	fmt.Printf("Onion Address: %s.onion\n", onion.ID)
	fmt.Printf("\n%sWaiting for peer connection...%s\n", ColorGray, ColorReset)

	conn, err := onion.Accept()
	if err != nil {
		log.Panicf("Accept failed: %v", err)
	}

	secureStream(conn)
}

func runClient(t *tor.Tor, address string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Dialing requires the SOCKS port that bine just automatically assigned
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

	secureStream(conn)
}

func secureStream(conn net.Conn) {
	defer conn.Close()

	// --- HANDSHAKE (ECDH) ---
	fmt.Printf("%sPerforming Diffie-Hellman Key Exchange...%s\n", ColorYellow, ColorReset)

	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		log.Panic(err)
	}
	pubKey := privKey.PublicKey()

	b64PubKey := base64.StdEncoding.EncodeToString(pubKey.Bytes())
	fmt.Fprintf(conn, "%s\n", b64PubKey)

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		log.Panic("Handshake failed: Peer disconnected")
	}

	peerPubKeyBytes, err := base64.StdEncoding.DecodeString(scanner.Text())
	if err != nil {
		log.Panic("Handshake failed: Invalid key format")
	}

	peerPubKey, err := curve.NewPublicKey(peerPubKeyBytes)
	if err != nil {
		log.Panic("Handshake failed: Invalid key curve")
	}

	sharedSecret, err := privKey.ECDH(peerPubKey)
	if err != nil {
		log.Panic("Handshake failed: Could not compute secret")
	}

	aesKey := sha256.Sum256(sharedSecret)
	fingerprint := fmt.Sprintf("%x", aesKey)[:16]

	fmt.Printf("\n%s--------------------------------------------------%s\n", ColorYellow, ColorReset)
	fmt.Printf(" %sENCRYPTED SESSION ESTABLISHED (AES-256-GCM)%s\n", ColorGreen, ColorReset)
	fmt.Printf(" SAFETY FINGERPRINT: %s%s%s\n", ColorRed, fingerprint, ColorReset)
	fmt.Printf(" (Verify this matches your peer!)\n")
	fmt.Printf("%s--------------------------------------------------%s\n\n", ColorYellow, ColorReset)

	// --- CHAT ---
	block, err := aes.NewCipher(aesKey[:])
	if err != nil {
		log.Panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Panic(err)
	}

	done := make(chan struct{})

	// Incoming
	go func() {
		for scanner.Scan() {
			encryptedMsg, err := base64.StdEncoding.DecodeString(scanner.Text())
			if err != nil {
				continue
			}

			nonceSize := gcm.NonceSize()
			if len(encryptedMsg) < nonceSize {
				continue
			}
			nonce, ciphertext := encryptedMsg[:nonceSize], encryptedMsg[nonceSize:]

			plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
			if err != nil {
				fmt.Printf("%s[Decryption Failed]%s\n", ColorRed, ColorReset)
				continue
			}

			ts := time.Now().Format("15:04")
			fmt.Printf("\r%s[%s] <Peer>: %s%s\n> ", ColorGreen, ts, string(plaintext), ColorReset)
		}
		fmt.Printf("\n%s[Peer disconnected]%s\n", ColorRed, ColorReset)
		close(done)
	}()

	// Outgoing
	go func() {
		inputScanner := bufio.NewScanner(os.Stdin)
		fmt.Print("> ")
		for inputScanner.Scan() {
			msg := inputScanner.Text()
			fmt.Printf("\033[1A\033[K") // UI cleanup
			ts := time.Now().Format("15:04")
			fmt.Printf("%s[%s] <Me>: %s%s\n", ColorCyan, ts, msg, ColorReset)

			nonce := make([]byte, gcm.NonceSize())
			rand.Read(nonce)
			ciphertext := gcm.Seal(nonce, nonce, []byte(msg), nil)
			out := base64.StdEncoding.EncodeToString(ciphertext)
			fmt.Fprintf(conn, "%s\n", out)
			fmt.Print("> ")
		}
		conn.Close()
	}()

	<-done
}