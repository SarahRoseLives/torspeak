# Torspeak

Torspeak is an ephemeral, end-to-end encrypted chat tool designed for private communication. It routes all traffic through the Tor network to protect your location (IP address) and leverages AES-256-GCM encryption, ensuring that not even Tor exit nodes can read your messages.

**Important:**  
When a session ends, encryption keys are securely wiped. Each generated onion address is one-time use—once destroyed, it cannot be reused.

---

## Features

- **No Location Leak:** Communication is routed via Tor, hiding your IP address.
- **Ephemeral Sessions:** Each chat session uses a unique Tor onion address.
- **End-to-End Encryption:** Messages are encrypted using AES-256-GCM.
- **Key Destruction:** Keys are wiped at session end for maximum privacy.

---

## Getting Started

### 1. Starting a Chat (Host)

To initiate a conversation:

1. Open your terminal.
2. Start Torspeak as host:
   ```bash
   ./torspeak host
   ```
3. Wait ~10–30 seconds for the Tor circuit to bootstrap.
4. When ready, you'll see:
   ```
   SECURE LINE READY
   Onion Address: v2c3...d4.onion
   ```
5. **Share your onion address with your friend securely** (e.g., via Signal, encrypted email, or PGP).
6. Wait for your friend to connect.

---

### 2. Joining a Chat (Guest)

If you have received an onion address:

1. Open your terminal.
2. Connect to the host with:
   ```bash
   ./torspeak connect v2c3...d4.onion
   ```
3. Wait. A connection to the hidden service may take a few moments.

---

### 3. Security Verification (Crucial)

**Do not chat immediately!**

Both participants will see a safety fingerprint displayed in **RED**:
```
--------------------------------------------------
 ENCRYPTED SESSION ESTABLISHED (AES-256-GCM)
 SAFETY FINGERPRINT: 8f4b2e19a0c3d4...
 (Verify this matches your peer!)
--------------------------------------------------
```

**Action:**  
- Before chatting, confirm the fingerprint matches your partner’s. Read several characters aloud over a different channel or compare them securely.
- **If fingerprints match:** Your connection is secure!
- **If fingerprints do NOT match:** Disconnect—this may indicate a "Man-in-the-Middle" attack.

---

### 4. Chatting

- **You:** Messages appear in _Cyan_.
- **Peer:** Messages appear in _Green_.
- **Timestamps:** All messages are labeled with your local time.

---

### 5. Ending the Session

- To close the chat, press <kbd>Ctrl</kbd>+<kbd>C</kbd>.
- You will see:  
  `[!] Wiping session data and keys...`
- The onion address is permanently destroyed. To chat again, restart the program as host to generate a new address.

---

## Troubleshooting

- **"Address already in use":**  
  Another instance of Torspeak is running. Make sure previous sessions are closed.

- **Slow Connection:**  
  The Tor network prioritizes privacy over speed; expect 1–3 seconds per message.

- **Stuck on "Starting Tor":**  
  First run downloads the Tor binary. On a slow internet connection, allow up to a minute.

---

## Security Notes

- Always verify the safety fingerprint before chatting.
- Never use the same onion address for multiple sessions.
- Keys and session data are never persisted after disconnecting.

