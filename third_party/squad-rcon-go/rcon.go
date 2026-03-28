package rcon

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/iamalone98/eventEmitter"

	"github.com/SquadGO/squad-rcon-go/v2/internal/parser"
	"github.com/SquadGO/squad-rcon-go/v2/internal/utils"
	"github.com/SquadGO/squad-rcon-go/v2/rconEvents"
)

const (
	serverDataAuth     = 0x03
	serverDataCommand  = 0x02
	serverDataServer   = 0x01
	serverDataResponse = 0x00

	emptyPacketID    = 100
	authPacketID     = 101
	executeCommandID = 50
)

var (
	executeWaitTimeout   = 5 * time.Second
	readDeadlineTimeout  = 100 * time.Millisecond
	writeDeadlineTimeout = 5 * time.Second
	tcpKeepAlivePeriod   = 30 * time.Second
)

type RconConfig struct {
	Host               string
	Port               string
	Password           string
	AutoReconnect      bool
	AutoReconnectDelay int
}

type Rcon struct {
	Emitter            eventEmitter.EventEmitter
	connected          bool
	reconnecting       bool
	client             net.Conn
	host               string
	port               string
	password           string
	responseBody       string
	autoReconnect      bool
	autoReconnectDelay int
	lastDataBuffer     []byte
	executeChan        chan string
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	mu                 sync.RWMutex
	executeMu          sync.Mutex
}

func NewRcon(config RconConfig) (*Rcon, error) {
	return NewRconWithContext(context.Background(), config)
}

func NewRconWithContext(ctx context.Context, config RconConfig) (*Rcon, error) {
	rconCtx, cancel := context.WithCancel(ctx)

	r := &Rcon{
		Emitter:            eventEmitter.NewEventEmitter(),
		host:               config.Host,
		port:               config.Port,
		password:           config.Password,
		connected:          false,
		lastDataBuffer:     make([]byte, 0),
		executeChan:        make(chan string),
		autoReconnect:      config.AutoReconnect,
		autoReconnectDelay: config.AutoReconnectDelay,
		ctx:                rconCtx,
		cancel:             cancel,
	}

	r.Emitter.On(rconEvents.ERROR, func(i interface{}) {
		// Check if context is cancelled to avoid unnecessary reconnection attempts
		select {
		case <-rconCtx.Done():
			return
		default:
		}

		r.mu.Lock()
		wasConnected := r.connected
		r.connected = false
		shouldReconnect := r.autoReconnect && r.autoReconnectDelay > 0 && !r.reconnecting && wasConnected
		r.mu.Unlock()

		if shouldReconnect {
			r.reconnect()
		}
	})

	if err := r.connect(); err != nil {
		cancel()
		return nil, err
	}

	return r, nil
}

func (r *Rcon) Close() {
	// First cancel context to signal all goroutines to stop
	r.cancel()

	// Close the connection to unblock any pending reads/writes
	r.mu.Lock()
	if r.client != nil {
		r.client.Close()
	}
	wasConnected := r.connected
	r.connected = false
	r.mu.Unlock()

	// Wait for all goroutines to finish
	r.wg.Wait()

	// Clean up resources
	r.reset()

	if wasConnected {
		r.Emitter.Emit(rconEvents.CLOSE, true)
	}
}

func (r *Rcon) Execute(command string) string {
	// Check context first to avoid unnecessary operations if closing
	select {
	case <-r.ctx.Done():
		return ""
	default:
	}

	r.executeMu.Lock()
	defer r.executeMu.Unlock()

	r.mu.RLock()
	if !r.connected || r.client == nil {
		r.mu.RUnlock()
		return ""
	}
	client := r.client
	r.mu.RUnlock()

	// Use context-aware writes
	if err := r.writeWithContext(client, utils.Encode(serverDataCommand, executeCommandID, command)); err != nil {
		return ""
	}
	if err := r.writeWithContext(client, utils.Encode(serverDataCommand, emptyPacketID, "")); err != nil {
		return ""
	}

	select {
	case v := <-r.executeChan:
		return v
	case <-time.After(executeWaitTimeout):
		r.Emitter.Emit(rconEvents.ERROR, fmt.Errorf("[RCON] Command timeout waiting for response: %s", command))
		return ""
	case <-r.ctx.Done():
		return ""
	}
}

func (r *Rcon) writeWithContext(conn net.Conn, data []byte) error {
	select {
	case <-r.ctx.Done():
		return fmt.Errorf("context cancelled")
	default:
	}

	if err := conn.SetWriteDeadline(time.Now().Add(writeDeadlineTimeout)); err != nil {
		wrappedErr := fmt.Errorf("[RCON] Failed to set write deadline: %w", err)
		r.Emitter.Emit(rconEvents.ERROR, wrappedErr)
		return wrappedErr
	}
	defer conn.SetWriteDeadline(time.Time{})

	written, err := conn.Write(data)
	if err == nil && written != len(data) {
		err = fmt.Errorf("short write: wrote %d of %d bytes", written, len(data))
	}
	if err != nil {
		wrappedErr := fmt.Errorf("[RCON] Command write error: %w", err)
		r.Emitter.Emit(rconEvents.ERROR, wrappedErr)
		return wrappedErr
	}

	return err
}

func (r *Rcon) connect() error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", r.host, r.port), 5*time.Second)

	if err != nil {
		msg := fmt.Errorf("[RCON] Connection error: %w", err)
		r.Emitter.Emit(rconEvents.ERROR, msg)
		return msg
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			_ = conn.Close()
			msg := fmt.Errorf("[RCON] Failed to enable TCP keepalive: %w", err)
			r.Emitter.Emit(rconEvents.ERROR, msg)
			return msg
		}
		if err := tcpConn.SetKeepAlivePeriod(tcpKeepAlivePeriod); err != nil {
			_ = conn.Close()
			msg := fmt.Errorf("[RCON] Failed to set TCP keepalive period: %w", err)
			r.Emitter.Emit(rconEvents.ERROR, msg)
			return msg
		}
	}

	r.mu.Lock()
	r.client = conn
	r.mu.Unlock()

	if err := r.auth(); err != nil {
		_ = conn.Close()
		r.mu.Lock()
		if r.client == conn {
			r.client = nil
		}
		r.mu.Unlock()
		return err
	}

	r.wg.Add(1)
	go r.byteReader()

	r.mu.Lock()
	r.connected = true
	r.reconnecting = false
	r.mu.Unlock()

	r.Emitter.Emit(rconEvents.CONNECTED, true)

	return nil
}

func (r *Rcon) auth() error {
	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()

	if client == nil {
		msg := fmt.Errorf("[RCON] No client connection available")
		r.Emitter.Emit(rconEvents.ERROR, msg)
		return msg
	}

	if err := r.writeWithContext(client, utils.Encode(serverDataAuth, authPacketID, r.password)); err != nil {
		msg := fmt.Errorf("[RCON] Authorization error: %w", err)
		r.Emitter.Emit(rconEvents.ERROR, msg)
		return msg
	}

	return nil
}

func (r *Rcon) reconnect() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ticker := time.NewTicker(time.Duration(r.autoReconnectDelay) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				r.mu.RLock()
				connected := r.connected
				reconnecting := r.reconnecting
				r.mu.RUnlock()

				if connected {
					return
				}

				// Prevent multiple concurrent reconnection attempts
				if reconnecting {
					continue
				}

				r.Emitter.Emit(rconEvents.RECONNECTING, true)
				r.mu.Lock()
				r.reconnecting = true
				client := r.client
				r.client = nil
				r.mu.Unlock()

				if client != nil {
					_ = client.Close()
				}

				r.reset()
				if err := r.connect(); err != nil {
					// Reset reconnecting flag on failed connection attempt
					r.mu.Lock()
					r.reconnecting = false
					r.mu.Unlock()
				}
			}
		}
	}()
}

func (r *Rcon) byteReader() {
	defer r.wg.Done()

	r.mu.RLock()
	client := r.client
	r.mu.RUnlock()

	if client == nil {
		r.Emitter.Emit(rconEvents.ERROR, fmt.Errorf("[RCON] No client connection available"))
		return
	}

	reader := bufio.NewReader(client)

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// Set a read deadline to prevent blocking indefinitely
			if err := client.SetReadDeadline(time.Now().Add(readDeadlineTimeout)); err != nil {
				select {
				case <-r.ctx.Done():
					return
				default:
				}

				r.Emitter.Emit(rconEvents.ERROR, fmt.Errorf("[RCON] Failed to set read deadline: %w", err))
				return
			}

			b, e := reader.ReadByte()
			if e != nil {
				// Check if it's a timeout error
				if netErr, ok := e.(net.Error); ok && netErr.Timeout() {
					continue // Continue the loop to check context cancellation
				}

				var err error
				if errors.Is(e, syscall.ECONNRESET) {
					err = fmt.Errorf("[RCON] Error: %w. Check password", e)
				} else if errors.Is(e, syscall.EADDRNOTAVAIL) {
					err = fmt.Errorf("[RCON] Error: %w. Connection lost", e)
				} else {
					err = fmt.Errorf("[RCON] Unknown error: %w", e)
				}

				r.Emitter.Emit(rconEvents.ERROR, err)
				return
			}

			r.byteParser(b)
		}
	}
}

func (r *Rcon) byteParser(b byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastDataBuffer = append(r.lastDataBuffer, b)

	if len(r.lastDataBuffer) >= 7 {
		size := int32(binary.LittleEndian.Uint32(r.lastDataBuffer[:4])) + 4

		if r.lastDataBuffer[0] == 0 &&
			r.lastDataBuffer[1] == 1 &&
			r.lastDataBuffer[2] == 0 &&
			r.lastDataBuffer[3] == 0 &&
			r.lastDataBuffer[4] == 0 &&
			r.lastDataBuffer[5] == 0 &&
			r.lastDataBuffer[6] == 0 {

			parser.RconParser(r.responseBody, r.Emitter)

			// Safely send response, handling closed context
			select {
			case r.executeChan <- r.responseBody:
			case <-r.ctx.Done():
				return
			default:
				// Channel might be full or receiver gone, skip
			}

			r.responseBody = ""
			r.lastDataBuffer = make([]byte, 0)
		}

		if int32(len(r.lastDataBuffer)) == size {
			packet := utils.Decode(r.lastDataBuffer)

			if packet.Type == serverDataResponse && packet.ID != authPacketID && packet.ID != emptyPacketID {
				r.responseBody += packet.Body
			}

			if packet.Type == serverDataServer {
				parser.RconParser(packet.Body, r.Emitter)
			}

			r.lastDataBuffer = r.lastDataBuffer[size:]
		}
	}
}

func (r *Rcon) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastDataBuffer = make([]byte, 0)
	r.responseBody = ""
}
