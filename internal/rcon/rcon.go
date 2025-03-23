package rcon

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"go.codycody31.dev/squad-aegis/internal/eventEmitter"

	"go.codycody31.dev/squad-aegis/internal/rcon/internal/utils"
	p "go.codycody31.dev/squad-aegis/internal/rcon/parser"
)

const (
	serverDataExecCommand   = 0x02
	serverDataResponseValue = 0x00
	serverDataAuth          = 0x03
	serverDataAuthResponse  = 0x02
	serverDataChatValue     = 0x01

	midPacketID = 0x01
	endPacketID = 0x02
)

type Warn p.Warn
type Kick p.Kick
type Message p.Message
type CommandMessage p.CommandMessage
type PosAdminCam p.PosAdminCam
type UnposAdminCam p.UnposAdminCam
type SquadCreated p.SquadCreated
type Players p.Players
type Squads p.Squads

type RconConfig struct {
	Host               string
	Port               string
	Password           string
	AutoReconnect      bool
	AutoReconnectDelay int
}

type Rcon struct {
	Emitter            eventEmitter.EventEmitter
	Connected          bool
	Reconnecting       bool
	client             net.Conn
	host               string
	port               string
	password           string
	responseBody       string
	lastCommand        string
	autoReconnect      bool
	autoReconnectDelay int
	lastDataBuffer     []byte
	executeChan        chan string
	done               chan struct{}
}

func NewRcon(config RconConfig) (*Rcon, error) {
	r := &Rcon{
		Emitter:            eventEmitter.NewEventEmitter(),
		host:               config.Host,
		port:               config.Port,
		password:           config.Password,
		Connected:          false,
		lastDataBuffer:     make([]byte, 0),
		executeChan:        make(chan string),
		autoReconnect:      config.AutoReconnect,
		autoReconnectDelay: config.AutoReconnectDelay,
		done:               make(chan struct{}),
	}

	if err := r.connect(); err != nil {
		return nil, err
	}

	if err := r.auth(); err != nil {
		return nil, err
	}

	go r.byteReader()

	r.ping()

	return r, nil
}

func (r *Rcon) Close() {
	if r.Connected {
		r.Connected = false

		r.lastCommand = ""
		r.lastDataBuffer = make([]byte, 0)

		close(r.executeChan)
		r.client.Close()

		r.Emitter.Emit("close", true)

		// Signal goroutines to stop
		close(r.done)

		if r.autoReconnect && r.autoReconnectDelay > 0 {
			r.reconnect(r.autoReconnectDelay)
		}
	}
}

func (r *Rcon) Execute(command string) (string, error) {
	return r.write(serverDataExecCommand, command)
}

func (r *Rcon) write(t int, data string) (string, error) {
	if !r.Connected {
		return "", errors.New("not connected")
	}

	var packetId int

	if t == serverDataAuth {
		packetId = endPacketID
	} else {
		packetId = midPacketID
	}

	encodedPacket := utils.Encode(t, packetId, data)
	encodedEmptyPacket := utils.Encode(t, endPacketID, "")

	if len(encodedPacket) > 4096 {
		return "", errors.New("packet too large")
	}

	r.client.Write(encodedPacket)

	if t != serverDataAuth {
		r.client.Write(encodedEmptyPacket)
	}

	r.lastCommand = data

	v, ok := <-r.executeChan
	if ok {
		return v, nil
	}

	if t == serverDataAuth {
		return "", errors.New("authorization failed")
	}

	return "", errors.New("no response")
}

func (r *Rcon) connect() error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", r.host, r.port), 5*time.Second)
	r.Reconnecting = false

	if err != nil {
		msg := fmt.Errorf("[RCON] Connection error: %w", err)
		r.Emitter.Emit("error", msg)
		return msg
	}

	r.client = conn
	r.Connected = true

	r.Emitter.Emit("connected", true)

	return nil
}

func (r *Rcon) auth() error {
	if _, err := r.client.Write(utils.Encode(0x03, 101, r.password)); err != nil {
		msg := fmt.Errorf("[RCON] Authorization error: %w", err)
		r.Emitter.Emit("error", msg)
		return msg
	}

	return nil
}

func (r *Rcon) reconnect(delay int) {
	ticker := time.NewTicker(time.Duration(delay) * time.Second)
	go func() {
	loop:
		for {
			select {
			case <-ticker.C:
				if r.Connected {
					break loop
				}

				if !r.Reconnecting {
					r.Reconnecting = true
					r.connect()
				}
			}
		}
	}()
}

func (r *Rcon) ping() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if r.Connected {
					r.Execute("PING_CONNECTION")
				}
			case <-r.done:
				return
			}
		}
	}()
}

func (r *Rcon) byteReader() {
	var err error
	reader := bufio.NewReader(r.client)

	for {
		select {
		case <-r.done:
			return
		default:
			b, e := reader.ReadByte()
			if e != nil {
				if errors.Is(e, syscall.ECONNRESET) {
					err = fmt.Errorf("[RCON] Error: %w. Check password", e)
				} else if errors.Is(e, syscall.EADDRNOTAVAIL) {
					err = fmt.Errorf("[RCON] Error: %w. Connection lost", e)
				} else {
					err = fmt.Errorf("[RCON] Unknown error: %w", e)
				}

				break
			}

			r.byteParser(b)
		}
	}

	r.Emitter.Emit("error", err)
	r.Close()
}

func (r *Rcon) byteParser(b byte) {
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

			switch data := p.CommandParser(r.responseBody, r.lastCommand).(type) {
			case p.Players:
				{
					r.Emitter.Emit("ListPlayers", Players(data))
				}
			case p.Squads:
				{
					r.Emitter.Emit("ListSquads", Squads(data))
				}
			}

			r.executeChan <- r.responseBody
			r.responseBody = ""
			r.lastDataBuffer = make([]byte, 0)
		}

		if int32(len(r.lastDataBuffer)) == size {
			packet := utils.Decode(r.lastDataBuffer)
			if packet.Type == 0x00 && packet.ID != 101 && packet.ID != 100 {
				r.responseBody += packet.Body
			}

			if packet.Type == 0x01 {
				r.Emitter.Emit("data", packet.Body)

				switch data := p.ChatParser(packet.Body).(type) {
				case p.Warn:
					{
						r.Emitter.Emit("PLAYER_WARNED", Warn(data))
					}
				case p.Kick:
					{
						r.Emitter.Emit("PLAYER_KICKED", Kick(data))
					}
				case p.Message:
					{
						r.Emitter.Emit("CHAT_MESSAGE", Message(data))
					}
				case p.CommandMessage:
					{
						r.Emitter.Emit("CHAT_COMMAND", CommandMessage(data))
					}
				case p.PosAdminCam:
					{
						r.Emitter.Emit("POSSESSED_ADMIN_CAMERA", PosAdminCam(data))
					}
				case p.UnposAdminCam:
					{
						r.Emitter.Emit("UNPOSSESSED_ADMIN_CAMERA", UnposAdminCam(data))
					}
				case p.SquadCreated:
					{
						r.Emitter.Emit("SQUAD_CREATED", SquadCreated(data))
					}
				}
			}

			r.lastDataBuffer = r.lastDataBuffer[size:]
		}
	}
}
