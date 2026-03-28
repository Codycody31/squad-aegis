package rcon

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SquadGO/squad-rcon-go/v2/rconEvents"
	"github.com/iamalone98/eventEmitter"
)

type fakeAddr string

func (a fakeAddr) Network() string { return "test" }
func (a fakeAddr) String() string  { return string(a) }

type fakeConn struct {
	writeFunc func([]byte) (int, error)
}

func (f *fakeConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (f *fakeConn) Write(p []byte) (int, error)        { return f.writeFunc(p) }
func (f *fakeConn) Close() error                       { return nil }
func (f *fakeConn) LocalAddr() net.Addr                { return fakeAddr("local") }
func (f *fakeConn) RemoteAddr() net.Addr               { return fakeAddr("remote") }
func (f *fakeConn) SetDeadline(_ time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(_ time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(_ time.Time) error { return nil }

func TestExecuteSerializesConcurrentCalls(t *testing.T) {
	originalTimeout := executeWaitTimeout
	executeWaitTimeout = 20 * time.Millisecond
	defer func() {
		executeWaitTimeout = originalTimeout
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	activeWrites := 0
	maxActiveWrites := 0
	releaseWrites := make(chan struct{})

	conn := &fakeConn{
		writeFunc: func(p []byte) (int, error) {
			mu.Lock()
			activeWrites++
			if activeWrites > maxActiveWrites {
				maxActiveWrites = activeWrites
			}
			mu.Unlock()

			<-releaseWrites

			mu.Lock()
			activeWrites--
			mu.Unlock()

			return len(p), nil
		},
	}

	r := &Rcon{
		Emitter:     eventEmitter.NewEventEmitter(),
		connected:   true,
		client:      conn,
		executeChan: make(chan string),
		ctx:         ctx,
		cancel:      cancel,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = r.Execute("ListPlayers")
	}()
	go func() {
		defer wg.Done()
		_ = r.Execute("ListSquads")
	}()

	time.Sleep(10 * time.Millisecond)
	close(releaseWrites)
	wg.Wait()

	if maxActiveWrites != 1 {
		t.Fatalf("expected Execute to serialize writes, saw max concurrent writes = %d", maxActiveWrites)
	}
}

func TestExecuteTimeoutEmitsError(t *testing.T) {
	originalTimeout := executeWaitTimeout
	executeWaitTimeout = 20 * time.Millisecond
	defer func() {
		executeWaitTimeout = originalTimeout
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &fakeConn{
		writeFunc: func(p []byte) (int, error) {
			return len(p), nil
		},
	}

	r := &Rcon{
		Emitter:     eventEmitter.NewEventEmitter(),
		connected:   true,
		client:      conn,
		executeChan: make(chan string),
		ctx:         ctx,
		cancel:      cancel,
	}

	errCh := make(chan error, 1)
	r.Emitter.On(rconEvents.ERROR, func(data interface{}) {
		if err, ok := data.(error); ok {
			select {
			case errCh <- err:
			default:
			}
		}
	})

	if response := r.Execute("ListPlayers"); response != "" {
		t.Fatalf("expected empty response on timeout, got %q", response)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatalf("expected timeout error to be emitted")
		}
		if got := err.Error(); got == "" || !strings.Contains(got, "Command timeout waiting for response") {
			t.Fatalf("expected timeout error message, got %q", got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected Execute timeout to emit an error event")
	}
}
