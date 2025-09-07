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

	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/internal/parser"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/internal/utils"
	"go.codycody31.dev/squad-aegis/internal/squad-rcon-go/rconEvents"
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
		r.mu.Lock()
		r.connected = false
		r.mu.Unlock()

		if r.autoReconnect && r.autoReconnectDelay > 0 && !r.reconnecting {
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
	r.mu.Lock()
	if r.connected {
		r.connected = false
		r.mu.Unlock()

		// Cancel context to signal all goroutines to stop
		r.cancel()

		// Wait for all goroutines to finish
		r.wg.Wait()

		r.reset()
		if r.client != nil {
			r.client.Close()
		}

		r.Emitter.Emit(rconEvents.CLOSE, true)
	} else {
		r.mu.Unlock()
	}
}

func (r *Rcon) Execute(command string) string {
	r.mu.RLock()
	if !r.connected || r.client == nil {
		r.mu.RUnlock()
		return ""
	}
	client := r.client
	r.mu.RUnlock()

	client.Write(utils.Encode(serverDataCommand, executeCommandID, command))
	client.Write(utils.Encode(serverDataCommand, emptyPacketID, ""))

	select {
	case v := <-r.executeChan:
		return v
	case <-time.After(5 * time.Second):
		return ""
	case <-r.ctx.Done():
		return ""
	}
}

func (r *Rcon) connect() error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", r.host, r.port), 5*time.Second)

	if err != nil {
		msg := fmt.Errorf("[RCON] Connection error: %w", err)
		r.Emitter.Emit(rconEvents.ERROR, msg)
		return msg
	}

	r.mu.Lock()
	r.client = conn
	r.mu.Unlock()

	if err := r.auth(); err != nil {
		return err
	}

	r.wg.Add(1)
	go r.byteReader()

	r.ping()

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

	if _, err := client.Write(utils.Encode(serverDataAuth, authPacketID, r.password)); err != nil {
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
				r.mu.RUnlock()

				if connected {
					return
				}

				r.Emitter.Emit(rconEvents.RECONNECTING, true)
				r.mu.Lock()
				r.reconnecting = true
				r.mu.Unlock()

				r.reset()
				r.connect()
			}
		}
	}()
}

func (r *Rcon) ping() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				r.mu.RLock()
				connected := r.connected
				r.mu.RUnlock()

				if connected {
					r.Execute("PING_CONNECTION")
				} else {
					return
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
			client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

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

			select {
			case r.executeChan <- r.responseBody:
			case <-r.ctx.Done():
				return
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
}
