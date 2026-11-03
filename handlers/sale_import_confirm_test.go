package handlers

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"afrita/config"
	"afrita/models"
)

// startRawBillBackend answers every request with the given raw HTTP response, after the request is read in full. A test uses it to
// send a 2xx whose body is malformed or cut off.
func startRawBillBackend(t *testing.T, response string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					return
				}
				_, _ = io.Copy(io.Discard, req.Body)
				_, _ = io.WriteString(conn, response)
			}()
		}
	}()
	previous := config.BackendDomain
	config.BackendDomain = "http://" + listener.Addr().String()
	t.Cleanup(func() { config.BackendDomain = previous })
}

// TestPostSaleBillNeedsAConfirmedBillID checks that a 2xx answer confirms a sale only when it is a bill id.
func TestPostSaleBillNeedsAConfirmedBillID(t *testing.T) {
	for _, body := range []string{"", "created", "0"} {
		startRawBillBackend(t, fmt.Sprintf("HTTP/1.1 201 Created\r\nContent-Length: %d\r\n\r\n%s", len(body), body))
		if id, err := postSaleBill("token", models.BillPayload{}); err == nil {
			t.Errorf("answer %q confirmed bill %d, want an unconfirmed row", body, id)
		}
	}

	startRawBillBackend(t, "HTTP/1.1 201 Created\r\nContent-Length: 3\r\n\r\n501")
	if id, err := postSaleBill("token", models.BillPayload{}); err != nil || id != 501 {
		t.Fatalf("answer 501: bill = %d, err = %v, want 501", id, err)
	}
}

// TestPostSaleBillRejectsAnAnswerThatIsCutOff checks that a 2xx answer that ends before its length is not a confirmation.
func TestPostSaleBillRejectsAnAnswerThatIsCutOff(t *testing.T) {
	startRawBillBackend(t, "HTTP/1.1 201 Created\r\nContent-Length: 10\r\n\r\n123")
	if id, err := postSaleBill("token", models.BillPayload{}); err == nil {
		t.Fatalf("a cut-off answer confirmed bill %d, want an unconfirmed row", id)
	}
}
