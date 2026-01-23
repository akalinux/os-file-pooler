package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
)

// force tls 1.3

func main() {
	// 1. Create a raw TCP listener and set it to non-blocking
	ln, _ := net.Listen("tcp", ":9002")
	tcpLn := ln.(*net.TCPListener)
	file, _ := tcpLn.File()
	fd := int(file.Fd())
	syscall.SetNonblock(fd, true)

	// 2. Load TLS Config
	cert, _ := tls.LoadX509KeyPair("server.crt", "server.key")
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

	fmt.Println("Server polling for async TLS connections on :9002...")

	for {
		// 3. Poll for new connections (Low-level Accept)
		nfd, _, err := syscall.Accept(fd)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				// No connection yet, continue polling or do other work
				continue
			}
			break
		}

		// 4. Handle the connection asynchronously
		go handleAsyncTLS(nfd, tlsConfig)
	}
}

func handleAsyncTLS(fd int, config *tls.Config) {
	syscall.SetNonblock(fd, true)

	f := os.NewFile(uintptr(fd), "")
	defer f.Close()
	conn, _ := net.FileConn(f)
	tlsConn := tls.Server(conn, config)

	for {
		err := tlsConn.Handshake()
		if err == nil {
			break // Handshake success
		}

		// REPLACEMENT FOR .Temporary(): Check for non-blocking "Retry" signals
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			// This is the "yielding" point. In a real event loop, you would
			// stop calling Handshake and wait for an epoll/kqueue notification.
			continue
		}

		// Check for actual timeouts if deadlines are set
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			fmt.Println("Handshake timed out")
			return
		}

		fmt.Println("Permanent handshake error:", err)
		return
	}

	fmt.Println("Asynchronous Handshake Complete")
}

type AsyncConn struct {
	conn    net.Conn  // The raw non-blocking socket
	tlsConn *tls.Conn // The TLS state machine
	readBuf []byte    // Persistent buffer for partial records
}

func (a *AsyncConn) HandleRead() ([]byte, error) {
	plaintext := make([]byte, 4096)

	// Attempt to read decrypted data
	n, err := a.tlsConn.Read(plaintext)

	if err != nil {
		// If we get EAGAIN, it means the TLS record is incomplete
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			// Return to poller and wait for more raw data
			return nil, nil
		}
		return nil, err
	}

	return plaintext[:n], nil
}

func (a *AsyncConn) HandleWrite(data []byte) (int, error) {
	// tlsConn.Write handles the fragmentation into TLS records.
	// In non-blocking mode, this might return a partial write.
	n, err := a.tlsConn.Write(data)

	if err != nil {
		if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) {
			// The OS buffer is full. You must save the 'n' offset
			// and retry writing data[n:] when the poller says 'Writable'.
			return n, nil
		}
		return n, err
	}
	return n, nil
}
