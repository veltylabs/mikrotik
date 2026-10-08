package tests

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/veltylabs/mikrotik"
	"github.com/veltylabs/mikrotik/routeros"
)

type breakableConn struct {
	routeros.Commander
	errBreak error // if set, returned on next Run
}

func (b *breakableConn) Run(sentence ...string) (routeros.Reply, error) {
	if b.errBreak != nil {
		err := b.errBreak
		b.errBreak = nil // break only once per configuration
		return routeros.Reply{}, err
	}
	return b.Commander.Run(sentence...)
}

func (b *breakableConn) Close() error {
	return nil
}

func TestSession(t *testing.T) {
	var dials int
	var currentConn *breakableConn
	var currentVersion string

	dial := func() (routeros.Conn, error) {
		dials++
		f := newEmulator(currentVersion)
		currentConn = &breakableConn{Commander: f}
		return currentConn, nil
	}

	// Case 1 & 4: Open does not dial and version switch
	currentVersion = "6.49.19 (stable)"
	gw := mikrotik.New(routeros.NewSession(dial))

	if dials != 0 {
		t.Fatalf("expected 0 dials on creation, got %d", dials)
	}

	// Dial on first use (current() called by Version)
	v, err := gw.Version()
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}
	if !strings.HasPrefix(v, "6.") {
		t.Fatalf("expected version 6.x, got %s", v)
	}

	if dials != 1 {
		t.Fatalf("expected 1 dial after Version(), got %d", dials)
	}

	// Case 2: Session dials once for two commands; after transport error NEXT command dials
	_, err = gw.Version() // second command, should not dial again
	if err != nil {
		t.Fatalf("failed to get version on 2nd call: %v", err)
	}
	if dials != 1 {
		t.Fatalf("expected still 1 dial, got %d", dials)
	}

	// Break the connection: the failing command is not retried and does not dial.
	currentConn.errBreak = errors.New("connection reset")
	_, err = gw.Connections()
	if err == nil || err.Error() != "connection reset" {
		t.Fatalf("expected connection reset error, got %v", err)
	}
	if dials != 1 {
		t.Fatalf("expected still 1 dial after broken command, got %d", dials)
	}

	// Case 4: the router came back upgraded. The very first operation after the
	// break dials again AND re-detects the version.
	currentVersion = "7.20.2 (stable)"
	v, err = gw.Version()
	if err != nil {
		t.Fatalf("failed to get version after reconnect: %v", err)
	}
	if !strings.HasPrefix(v, "7.") {
		t.Fatalf("expected version 7.x after reconnect, got %s", v)
	}
	if dials != 2 {
		t.Fatalf("expected 2 dials after reconnect, got %d", dials)
	}

	// Case 3: DeviceError keeps connection
	currentConn.errBreak = routeros.DeviceError{Path: "test", Err: errors.New("router error")}
	_, err = gw.Connections() // this will trigger the error
	if err == nil {
		t.Fatalf("expected device error, got nil")
	}

	// Next command should NOT dial
	_, err = gw.Connections()
	if err != nil {
		t.Fatalf("expected next command to succeed, got %v", err)
	}
	if dials != 2 {
		t.Fatalf("expected still 2 dials after DeviceError, got %d", dials)
	}
}

func TestSessionConcurrency(t *testing.T) {
	dial := func() (routeros.Conn, error) {
		f := newEmulator("6.49.19 (stable)")
		return &breakableConn{Commander: f}, nil
	}

	gw := mikrotik.New(routeros.NewSession(dial))

	// Concurrency: 20 goroutines calling Connections() on one Gateway
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Connections() triggers current() which triggers Session.Run()
			_, err := gw.Connections()
			if err != nil {
				t.Errorf("expected no error from Connections(), got %v", err)
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for concurrent calls")
	}
}
