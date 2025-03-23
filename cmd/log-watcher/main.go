package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/hpcloud/tail"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"

	pb "go.codycody31.dev/squad-aegis/proto/logwatcher"
)

// Auth token (configurable via CLI)
var authToken string

// LogWatcherServer implements the LogWatcher service
type LogWatcherServer struct {
	pb.UnimplementedLogWatcherServer
	mu        sync.Mutex
	clients   map[pb.LogWatcher_StreamLogsServer]struct{}
	eventSubs map[pb.LogWatcher_StreamEventsServer]struct{}
	logFile   string
}

// LogParser represents a log parser with a regex and a handler function
type LogParser struct {
	regex   *regexp.Regexp
	onMatch func([]string, *LogWatcherServer)
}

// Global parsers for structured events
var logParsers = []LogParser{
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: ADMIN COMMAND: Message broadcasted <(.+)> from (.+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched ADMIN_BROADCAST event: ", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":    args[1],
				"chainID": args[2],
				"message": args[3],
				"from":    args[4],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "ADMIN_BROADCAST",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQDeployable::)?TakeDamage\(\): ([A-z0-9_]+)_C_[0-9]+: ([0-9.]+) damage attempt by causer ([A-z0-9_]+)_C_[0-9]+ instigator (.+) with damage type ([A-z0-9_]+)_C health remaining ([0-9.]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched DEPLOYABLE_DAMAGED event: ", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":            args[1],
				"chainID":         args[2],
				"deployable":      args[3],
				"damage":          args[4],
				"weapon":          args[5],
				"playerSuffix":    args[6],
				"damageType":      args[7],
				"healthRemaining": args[8],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "DEPLOYABLE_DAMAGED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	// TODO: NEW_GAME
	// TODO: PLAYER_CONNECTED
	// TODO: PLAYER_DAMAGED
	// TODO: PLAYER_DIED
	// TODO: PLAYER_DISCONNECTED
	// TODO: JOIN_SUCCEEDED
	// TODO: PLAYER_POSSESS
	// TODO: PLAYER_REVIVED
	// TODO: PLAYER_UNPOSSESS
	// TODO: PLAYER_WOUNDED
	// TODO: ROUND_ENDED
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: USQGameState: Server Tick Rate: ([0-9.]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Println("[DEBUG] Matched TICK_RATE event: ", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":     args[1],
				"chainID":  args[2],
				"tickRate": args[3],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Println("[ERROR] Failed to marshal event data:", err)
				return
			}

			data := &pb.EventEntry{
				Event: "TICK_RATE",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
}

// NewLogWatcherServer initializes the server
func NewLogWatcherServer(logFile string) *LogWatcherServer {
	log.Println("[DEBUG] Initializing LogWatcherServer with file:", logFile)
	server := &LogWatcherServer{
		clients:   make(map[pb.LogWatcher_StreamLogsServer]struct{}),
		eventSubs: make(map[pb.LogWatcher_StreamEventsServer]struct{}),
		logFile:   logFile,
	}

	// Start processing logs for events
	go server.processLogFile()

	return server
}

// Authenticate using a simple token
func validateToken(tokenString string) bool {
	if tokenString == authToken {
		log.Println("[DEBUG] Authentication successful")
		return true
	}
	log.Println("[DEBUG] Authentication failed")
	return false
}

// StreamLogs streams raw log lines to authenticated clients
func (s *LogWatcherServer) StreamLogs(req *pb.AuthRequest, stream pb.LogWatcher_StreamLogsServer) error {
	if !validateToken(req.Token) {
		return fmt.Errorf("unauthorized")
	}

	s.mu.Lock()
	s.clients[stream] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, stream)
		s.mu.Unlock()
	}()

	// Start tailing logs
	t, err := tail.TailFile(s.logFile, tail.Config{Follow: true, ReOpen: true, Poll: true})
	if err != nil {
		return err
	}

	for line := range t.Lines {
		stream.Send(&pb.LogEntry{Content: strings.TrimSpace(line.Text)})
	}

	return nil
}

// StreamEvents streams structured events found in logs
func (s *LogWatcherServer) StreamEvents(req *pb.AuthRequest, stream pb.LogWatcher_StreamEventsServer) error {
	if !validateToken(req.Token) {
		return fmt.Errorf("unauthorized")
	}

	log.Println("[DEBUG] New StreamEvents subscriber")

	s.mu.Lock()
	s.eventSubs[stream] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.eventSubs, stream)
		s.mu.Unlock()
	}()

	// Keep stream open
	for {
		select {}
	}
}

// processLogFile continuously reads logs and processes events
func (s *LogWatcherServer) processLogFile() {
	// Start tailing logs
	t, err := tail.TailFile(s.logFile, tail.Config{Follow: true, ReOpen: true, Poll: true})
	if err != nil {
		return
	}

	for line := range t.Lines {
		s.processLogForEvents(line.Text)
	}
}

// processLogForEvents detects events based on regex and broadcasts them
func (s *LogWatcherServer) processLogForEvents(logLine string) {
	for _, parser := range logParsers {
		if matches := parser.regex.FindStringSubmatch(logLine); matches != nil {
			parser.onMatch(matches, s)
		}
	}
}

// broadcastEvent sends event data to all connected event stream clients
func (s *LogWatcherServer) broadcastEvent(event *pb.EventEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for stream := range s.eventSubs {
		stream.Send(event)
	}
}

// StartServer runs the gRPC server
func StartServer(c *cli.Context) error {
	logFile := c.String("log-file")
	port := c.String("port")
	authToken = c.String("auth-token")

	server := NewLogWatcherServer(logFile)

	// Start gRPC server
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterLogWatcherServer(grpcServer, server)

	log.Printf("[INFO] LogWatcher gRPC server listening on :%s", port)
	return grpcServer.Serve(lis)
}

func main() {
	app := &cli.App{
		Name:  "logwatcher",
		Usage: "Watches a file and streams changes via gRPC",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "log-file",
				Usage:    "Path to the log file to watch",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "port",
				Usage:    "Port to run the gRPC server on",
				Value:    "31135",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "auth-token",
				Usage:    "Simple auth token for authentication",
				Required: true,
			},
		},
		Action: StartServer,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
