package snpp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

var ErrFailedConnection = errors.New("SNPP Gateway did not accept connection")
var ErrRejectedPhone = errors.New("SNPP Gateway did not accept pager number")
var ErrRejectedMessage = errors.New("SNPP Gateway did not accept message")
var ErrFailedSend = errors.New("SNPP Gateway did not send message")
var ErrForceQuit = errors.New("SNPP Gateway did not close remote connection")

func read(conn net.Conn) (string, error) {
	// These gateways can be slow - so set a 30 second timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	var bigBuffer bytes.Buffer
	for {
		readBuf := make([]byte, 256)
		n, readErr := conn.Read(readBuf)
		if readErr != nil {
			if readErr != io.EOF {
				return "", readErr
			}
			break
		}

		bigBuffer.Write(readBuf[:n])

		// Messages end with a Carriage Return and a New Line
		if readBuf[n-2] == 13 && readBuf[n-1] == 10 {
			break
		}
	}

	return bigBuffer.String(), nil
}

func write(conn net.Conn, msg string) error {
	// These gateways can be slow - so set a 30 second timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	writer := bufio.NewWriter(conn)
	if _, writeErr := writer.WriteString(msg); writeErr != nil {
		return writeErr
	}

	return writer.Flush()
}

/*
 *  Following the simple example in section 4.1.1 https://tools.ietf.org/html/rfc1861
 *  1) Connect to SNPP gateway
 *  2) Wait for 220 gateway ready message
 *  3) Send PAGE command
 *  4) Wait for 250 success response
 *  5) Send MESS command
 *  6) Wait for 250 success response
 *  7) Send SEND COMMAND
 *  8) Wait for 250 success response
 *  9) Send QUIT COMMAND
 *  10) Wait for 221 goodbye response
 */
func SendPage(address string, port uint64, number string, message string) error {
	// Dial the snpp gateway. Give it a 30 second timeout because some of these providers are slooooow
	conn, connErr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), 30*time.Second)
	if connErr != nil {
		return connErr
	}
	defer conn.Close()

	msg, readErr := read(conn)
	if readErr != nil {
		return readErr
	}
	if !strings.HasPrefix(msg, "220") {
		return ErrFailedConnection
	}

	if writeErr := write(conn, fmt.Sprintf("PAGE %s \r\n", number)); writeErr != nil {
		return writeErr
	}
	msg, readErr = read(conn)
	if readErr != nil {
		return readErr
	}
	if !strings.HasPrefix(msg, "250") {
		return ErrRejectedPhone
	}

	if writeErr := write(conn, fmt.Sprintf("MESS %s \r\n", message)); writeErr != nil {
		return writeErr
	}
	msg, readErr = read(conn)
	if readErr != nil {
		return readErr
	}
	if !strings.HasPrefix(msg, "250") {
		return ErrRejectedMessage
	}

	if writeErr := write(conn, "SEND \r\n"); writeErr != nil {
		return writeErr
	}
	msg, readErr = read(conn)
	if readErr != nil {
		return readErr
	}
	if !strings.HasPrefix(msg, "250") {
		return ErrFailedSend
	}

	if writeErr := write(conn, "QUIT \r\n"); writeErr != nil {
		return writeErr
	}
	msg, readErr = read(conn)
	if readErr != nil {
		return readErr
	}
	if !strings.HasPrefix(msg, "221") {
		return ErrForceQuit
	}

	return nil
}
