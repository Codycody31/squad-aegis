package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/hpcloud/tail"
	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"github.com/urfave/cli/v3"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	pb "go.codycody31.dev/squad-aegis/proto/logwatcher"
	"go.codycody31.dev/squad-aegis/shared/logger"
	"go.codycody31.dev/squad-aegis/shared/utils"
)

// Auth token (configurable via CLI)
var authToken string

// LogSource defines an interface for different log sources
type LogSource interface {
	// Start watching logs and return a channel that receives log lines
	Watch(ctx context.Context) (<-chan string, error)
	// Close the log source
	Close() error
}

// LocalFileSource implements LogSource for local file access
type LocalFileSource struct {
	filepath string
	tail     *tail.Tail
}

// NewLocalFileSource creates a new local file source
func NewLocalFileSource(filepath string) *LocalFileSource {
	return &LocalFileSource{
		filepath: filepath,
	}
}

// Watch starts watching a local file for changes
func (l *LocalFileSource) Watch(ctx context.Context) (<-chan string, error) {
	cleanPath := filepath.Clean(l.filepath)
	t, err := tail.TailFile(cleanPath, tail.Config{
		Follow: true,
		ReOpen: true,
		Poll:   true,
	})
	if err != nil {
		return nil, err
	}
	l.tail = t

	logChan := make(chan string)
	go func() {
		defer close(logChan)
		for {
			select {
			case line := <-t.Lines:
				if line != nil {
					logChan <- strings.TrimSpace(line.Text)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return logChan, nil
}

// Close closes the local file source
func (l *LocalFileSource) Close() error {
	if l.tail != nil {
		return l.tail.Stop()
	}
	return nil
}

// SFTPSource implements LogSource for SFTP access
type SFTPSource struct {
	host           string
	port           int
	username       string
	password       string
	keyPath        string
	filepath       string
	client         *sftp.Client
	sshConn        *ssh.Client
	pollFreq       time.Duration
	lastPos        int64
	mu             sync.Mutex // Mutex for protecting client access
	reconnectDelay time.Duration
	maxDelay       time.Duration
	tempFilePath   string // Path to temporary file for downloads
	readFromStart  bool   // Whether to read from the beginning of the file
}

// NewSFTPSource creates a new SFTP source
func NewSFTPSource(host string, port int, username, password, keyPath, filepath string, pollFreq time.Duration, readFromStart bool) *SFTPSource {
	// Create a unique temp file name based on connection details
	h := fmt.Sprintf("%s:%d:%s", host, port, filepath)
	tempFileName := fmt.Sprintf("sftp-tail-%x.tmp", md5sum([]byte(h)))
	tempFilePath := os.TempDir() + "/" + tempFileName

	// Create temp directory if it doesn't exist
	if _, err := os.Stat(os.TempDir()); os.IsNotExist(err) {
		os.MkdirAll(os.TempDir(), 0755)
	}

	// Create temp file if it doesn't exist
	if _, err := os.Stat(tempFilePath); os.IsNotExist(err) {
		file, err := os.Create(tempFilePath)
		if err != nil {
			log.Printf("[ERROR] Failed to create temp file: %v", err)
		}
		file.Close()
	}

	return &SFTPSource{
		host:           host,
		port:           port,
		username:       username,
		password:       password,
		keyPath:        keyPath,
		filepath:       filepath,
		pollFreq:       pollFreq,
		lastPos:        0,
		reconnectDelay: 1 * time.Second,  // Start with 1 second
		maxDelay:       60 * time.Second, // Max 1 minute between retries
		tempFilePath:   tempFilePath,
		readFromStart:  readFromStart,
	}
}

// md5sum creates an MD5 hash of the input
func md5sum(data []byte) []byte {
	hash := md5.Sum(data)
	return hash[:]
}

// Watch starts watching an SFTP file for changes
func (s *SFTPSource) Watch(ctx context.Context) (<-chan string, error) {
	// Initial connection
	if err := s.connect(); err != nil {
		return nil, err
	}

	// Set initial position to end of file for proper tailing behavior
	if err := s.initializePosition(); err != nil {
		return nil, fmt.Errorf("failed to initialize file position: %v", err)
	}

	// Create temp file if it doesn't exist
	if _, err := os.Stat(s.tempFilePath); os.IsNotExist(err) {
		file, err := os.Create(s.tempFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %v", err)
		}
		file.Close()
	}

	logChan := make(chan string)

	// Start the polling goroutine
	go func() {
		defer close(logChan)
		defer os.Remove(s.tempFilePath) // Cleanup temp file when done

		ticker := time.NewTicker(s.pollFreq)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Test connection first
				if !s.isConnected() {
					log.Printf("[WARN] SFTP connection test failed, attempting reconnect")
					if err := s.reconnect(); err != nil {
						log.Printf("[ERROR] Failed to reconnect to SFTP server: %v", err)
						continue
					}
				}

				// Download new data and get lines
				newLines, err := s.fetchNewData()
				if err != nil {
					log.Printf("[ERROR] Failed to fetch data from SFTP: %v", err)
					// Try to reconnect on error
					if err := s.reconnect(); err != nil {
						log.Printf("[ERROR] Failed to reconnect to SFTP server: %v", err)
					}
					continue
				}

				// Reset reconnect delay after successful fetch
				s.mu.Lock()
				s.reconnectDelay = 1 * time.Second
				s.mu.Unlock()

				// Send each line to the channel
				for _, line := range newLines {
					logChan <- line
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return logChan, nil
}

// initializePosition sets the initial file position
func (s *SFTPSource) initializePosition() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if client is nil
	if s.client == nil {
		return fmt.Errorf("SFTP client is nil, connection may have been lost")
	}

	// Get file stats to set initial position
	stat, err := s.client.Stat(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to stat remote file: %v", err)
	}

	// Set initial position based on configuration
	fileSize := stat.Size()
	if s.readFromStart {
		s.lastPos = 0
		log.Printf("[INFO] Initial SFTP position set to start of file (reading from beginning)")
	} else {
		s.lastPos = fileSize
		log.Printf("[INFO] Initial SFTP position set to end of file (%d bytes)", fileSize)
	}

	return nil
}

// fetchNewData downloads new file content and returns parsed lines
func (s *SFTPSource) fetchNewData() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if client is nil
	if s.client == nil {
		return nil, fmt.Errorf("SFTP client is nil, connection may have been lost")
	}

	// Get file stats to check size
	stat, err := s.client.Stat(s.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat remote file: %v", err)
	}

	// Check if file size has changed
	fileSize := stat.Size()

	// If file size has not changed, return empty slice
	if fileSize == s.lastPos {
		return []string{}, nil
	}

	// If file has been rotated (smaller than our last position), reset position
	if fileSize < s.lastPos {
		log.Printf("[INFO] File size decreased from %d to %d, file may have been rotated", s.lastPos, fileSize)
		s.lastPos = 0
	}

	// Open remote file
	remoteFile, err := s.client.Open(s.filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file: %v", err)
	}
	defer remoteFile.Close()

	// Seek to last position
	_, err = remoteFile.Seek(s.lastPos, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to seek in remote file: %v", err)
	}

	// Open temporary file for writing
	tempFile, err := os.OpenFile(s.tempFilePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %v", err)
	}
	defer tempFile.Close()

	// Copy data from remote to temp file
	bytesRead, err := io.Copy(tempFile, remoteFile)
	if err != nil {
		return nil, fmt.Errorf("failed to copy data to temp file: %v", err)
	}

	// Update last position
	s.lastPos += bytesRead

	// If no new data, return empty result
	if bytesRead == 0 {
		return []string{}, nil
	}

	// Read the temp file content
	tempFile.Close() // Close before reading
	content, err := os.ReadFile(s.tempFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file: %v", err)
	}

	// Split content into lines
	contentStr := string(content)
	lines := strings.Split(strings.ReplaceAll(contentStr, "\r\n", "\n"), "\n")

	// Remove empty last line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines, nil
}

// isConnected tests if the SFTP connection is still valid
func (s *SFTPSource) isConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil || s.sshConn == nil {
		return false
	}

	// Try a simple operation - list directory of the parent folder
	parentDir := filepath.Dir(s.filepath)
	_, err := s.client.ReadDir(parentDir)
	return err == nil
}

// connect establishes an SFTP connection
func (s *SFTPSource) connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create SSH client configuration
	config := &ssh.ClientConfig{
		User:            s.username,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	// Add password auth if provided
	if s.password != "" {
		config.Auth = append(config.Auth, ssh.Password(s.password))
	}

	// Add key auth if provided
	if s.keyPath != "" {
		key, err := os.ReadFile(s.keyPath)
		if err != nil {
			return fmt.Errorf("unable to read private key: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("unable to parse private key: %v", err)
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	// Connect to SSH server
	sshConn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", s.host, s.port), config)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %v", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}

	// Close any existing connections
	if s.client != nil {
		s.client.Close()
	}
	if s.sshConn != nil {
		s.sshConn.Close()
	}

	// Store new connections
	s.sshConn = sshConn
	s.client = sftpClient

	return nil
}

// reconnect closes existing connections and creates new ones with exponential backoff
func (s *SFTPSource) reconnect() error {
	s.mu.Lock()
	delay := s.reconnectDelay
	// Implement exponential backoff
	if s.reconnectDelay*2 < s.maxDelay {
		s.reconnectDelay = s.reconnectDelay * 2
	} else {
		s.reconnectDelay = s.maxDelay
	}
	s.mu.Unlock()

	log.Printf("[INFO] Attempting to reconnect to SFTP server %s:%d (delay: %v)", s.host, s.port, delay)

	// Wait before reconnecting
	time.Sleep(delay)

	return s.connect()
}

// Close closes the SFTP source and removes temp file
func (s *SFTPSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errMsgs []string

	// Clean up temp file
	if err := os.Remove(s.tempFilePath); err != nil && !os.IsNotExist(err) {
		errMsgs = append(errMsgs, fmt.Sprintf("failed to remove temp file: %v", err))
	}

	if s.client != nil {
		if err := s.client.Close(); err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("failed to close SFTP client: %v", err))
		}
		s.client = nil
	}

	if s.sshConn != nil {
		if err := s.sshConn.Close(); err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("failed to close SSH connection: %v", err))
		}
		s.sshConn = nil
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf(strings.Join(errMsgs, "; "))
	}
	return nil
}

// FTPSource implements LogSource for FTP access
type FTPSource struct {
	host          string
	port          int
	username      string
	password      string
	filepath      string
	conn          *ftp.ServerConn
	pollFreq      time.Duration
	lastPos       int64
	mu            sync.Mutex    // Mutex for protecting connection access
	tempFilePath  string        // Path to temporary file for downloads
	maxRetries    int           // Maximum number of retries for operations
	retryDelay    time.Duration // Delay between retries
	readFromStart bool          // Whether to read from the beginning of the file
}

// NewFTPSource creates a new FTP source
func NewFTPSource(host string, port int, username, password, filepath string, pollFreq time.Duration, readFromStart bool) *FTPSource {
	// Create a unique temp file name based on connection details
	h := fmt.Sprintf("%s:%d:%s", host, port, filepath)
	tempFileName := fmt.Sprintf("ftp-tail-%x.tmp", md5sum([]byte(h)))
	tempFilePath := os.TempDir() + "/" + tempFileName

	// Create temp directory if it doesn't exist
	if _, err := os.Stat(os.TempDir()); os.IsNotExist(err) {
		os.MkdirAll(os.TempDir(), 0755)
	}

	// Create temp file if it doesn't exist
	if _, err := os.Stat(tempFilePath); os.IsNotExist(err) {
		file, err := os.Create(tempFilePath)
		if err != nil {
			log.Printf("[ERROR] Failed to create temp file: %v", err)
		}
		file.Close()
	}

	return &FTPSource{
		host:          host,
		port:          port,
		username:      username,
		password:      password,
		filepath:      filepath,
		pollFreq:      pollFreq,
		lastPos:       0,
		tempFilePath:  tempFilePath,
		maxRetries:    3,
		retryDelay:    time.Second,
		readFromStart: readFromStart,
	}
}

// Watch starts watching an FTP file for changes
func (f *FTPSource) Watch(ctx context.Context) (<-chan string, error) {
	// Initial connection
	if err := f.connect(); err != nil {
		return nil, err
	}

	// Set initial position to end of file for proper tailing behavior
	if err := f.initializePosition(); err != nil {
		return nil, fmt.Errorf("failed to initialize file position: %v", err)
	}

	// Create temp file if it doesn't exist
	if _, err := os.Stat(f.tempFilePath); os.IsNotExist(err) {
		file, err := os.Create(f.tempFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %v", err)
		}
		file.Close()
	}

	logChan := make(chan string)

	// Start the polling goroutine
	go func() {
		defer close(logChan)
		defer os.Remove(f.tempFilePath) // Cleanup temp file when done

		ticker := time.NewTicker(f.pollFreq)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Download new data and get lines
				newLines, err := f.fetchNewData()
				if err != nil {
					log.Printf("[ERROR] Failed to fetch data from FTP: %v", err)
					// Try to reconnect if connection was lost
					if err := f.reconnect(); err != nil {
						log.Printf("[ERROR] Failed to reconnect to FTP server: %v", err)
					}
					continue
				}

				// Send each line to the channel
				for _, line := range newLines {
					logChan <- line
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return logChan, nil
}

// initializePosition sets the initial file position
func (f *FTPSource) initializePosition() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if connection is nil
	if f.conn == nil {
		return fmt.Errorf("FTP connection is nil, connection may have been lost")
	}

	// Get file size with retries to set initial position
	var fileSize int64
	var err error

	for retry := 0; retry < f.maxRetries; retry++ {
		fileSize, err = f.conn.FileSize(f.filepath)
		if err == nil {
			break
		}

		if retry < f.maxRetries-1 {
			log.Printf("[WARN] Failed to get initial file size (attempt %d/%d): %v, retrying...",
				retry+1, f.maxRetries, err)
			time.Sleep(f.retryDelay)

			// Try reconnecting before retry
			if strings.Contains(err.Error(), "connection") {
				f.reconnect()
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get initial file size after %d attempts: %v",
			f.maxRetries, err)
	}

	// Set initial position based on configuration
	if f.readFromStart {
		f.lastPos = 0
		log.Printf("[INFO] Initial FTP position set to start of file (reading from beginning)")
	} else {
		f.lastPos = fileSize
		log.Printf("[INFO] Initial FTP position set to end of file (%d bytes)", fileSize)
	}

	return nil
}

// fetchNewData downloads new file content from FTP and returns parsed lines
func (f *FTPSource) fetchNewData() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if connection is nil
	if f.conn == nil {
		return nil, fmt.Errorf("FTP connection is nil, connection may have been lost")
	}

	// Get file size with retries
	var fileSize int64
	var err error

	for retry := 0; retry < f.maxRetries; retry++ {
		fileSize, err = f.conn.FileSize(f.filepath)
		if err == nil {
			break
		}

		if retry < f.maxRetries-1 {
			log.Printf("[WARN] Failed to get file size (attempt %d/%d): %v, retrying...",
				retry+1, f.maxRetries, err)
			time.Sleep(f.retryDelay)

			// Try reconnecting before retry
			if strings.Contains(err.Error(), "connection") {
				f.reconnect()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get file size after %d attempts: %v",
			f.maxRetries, err)
	}

	// If file size has not changed, return empty slice
	if f.lastPos == fileSize {
		return []string{}, nil
	}

	// If file has been rotated (smaller than our last position), reset position
	if f.lastPos > fileSize {
		log.Printf("[INFO] File size decreased from %d to %d, file may have been rotated",
			f.lastPos, fileSize)
		f.lastPos = 0
	}

	// Create a range command to read only new data (with retries)
	var resp *ftp.Response

	for retry := 0; retry < f.maxRetries; retry++ {
		resp, err = f.conn.RetrFrom(f.filepath, uint64(f.lastPos))
		if err == nil {
			break
		}

		if retry < f.maxRetries-1 {
			log.Printf("[WARN] Failed to retrieve file (attempt %d/%d): %v, retrying...",
				retry+1, f.maxRetries, err)
			time.Sleep(f.retryDelay)

			// Try reconnecting before retry
			if strings.Contains(err.Error(), "connection") {
				f.reconnect()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file after %d attempts: %v",
			f.maxRetries, err)
	}
	defer resp.Close()

	// Open temporary file for writing
	tempFile, err := os.OpenFile(f.tempFilePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %v", err)
	}
	defer tempFile.Close()

	// Copy data from remote to temp file
	bytesRead, err := io.Copy(tempFile, resp)
	if err != nil {
		return nil, fmt.Errorf("failed to copy data to temp file: %v", err)
	}

	// Update last position
	f.lastPos += bytesRead

	// If no new data, return empty result
	if bytesRead == 0 {
		return []string{}, nil
	}

	// Read the temp file content
	tempFile.Close() // Close before reading
	content, err := os.ReadFile(f.tempFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file: %v", err)
	}

	// Split content into lines
	contentStr := string(content)
	lines := strings.Split(strings.ReplaceAll(contentStr, "\r\n", "\n"), "\n")

	// Remove empty last line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines, nil
}

// connect establishes an FTP connection
func (f *FTPSource) connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Close existing connection if any
	if f.conn != nil {
		f.conn.Quit()
		f.conn = nil
	}

	// Connect to FTP server
	addr := fmt.Sprintf("%s:%d", f.host, f.port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("failed to connect to FTP server: %v", err)
	}

	// Login
	if err := conn.Login(f.username, f.password); err != nil {
		conn.Quit()
		return fmt.Errorf("failed to login to FTP server: %v", err)
	}

	f.conn = conn
	return nil
}

// reconnect attempts to reconnect to the FTP server
func (f *FTPSource) reconnect() error {
	log.Printf("[INFO] Attempting to reconnect to FTP server %s:%d", f.host, f.port)
	return f.connect()
}

// Close closes the FTP source and removes temp file
func (f *FTPSource) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errMsgs []string

	// Clean up temp file
	if err := os.Remove(f.tempFilePath); err != nil && !os.IsNotExist(err) {
		errMsgs = append(errMsgs, fmt.Sprintf("failed to remove temp file: %v", err))
	}

	if f.conn != nil {
		err := f.conn.Quit()
		f.conn = nil
		if err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("failed to quit FTP connection: %v", err))
		}
	}

	if len(errMsgs) > 0 {
		return fmt.Errorf(strings.Join(errMsgs, "; "))
	}
	return nil
}

// EventStore tracks player data across the server session
type EventStore struct {
	mu           sync.RWMutex
	disconnected map[string]map[string]interface{} // Players who disconnected, cleared on map change
	players      map[string]map[string]interface{} // Persistent player data (steamId, controller, suffix)
	session      map[string]map[string]interface{} // Non-persistent session data
	joinRequests map[string]map[string]interface{} // Track join requests by chainID
}

// NewEventStore creates a new event store
func NewEventStore() *EventStore {
	return &EventStore{
		disconnected: make(map[string]map[string]interface{}),
		players:      make(map[string]map[string]interface{}),
		session:      make(map[string]map[string]interface{}),
		joinRequests: make(map[string]map[string]interface{}),
	}
}

// LogWatcherServer implements the LogWatcher service
type LogWatcherServer struct {
	pb.UnimplementedLogWatcherServer
	mu         sync.Mutex
	clients    map[pb.LogWatcher_StreamLogsServer]struct{}
	eventSubs  map[pb.LogWatcher_StreamEventsServer]struct{}
	logSource  LogSource
	eventStore *EventStore
	ctx        context.Context
	cancel     context.CancelFunc
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
			log.Info().Msgf("Matched ADMIN_BROADCAST event: %v", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":    args[1],
				"chainID": args[2],
				"message": args[3],
				"from":    args[4],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
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
			log.Info().Msgf("Matched DEPLOYABLE_DAMAGED event: %v", args)
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
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "DEPLOYABLE_DAMAGED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: PostLogin: NewPlayer: BP_PlayerController_C .+PersistentLevel\.([^\s]+) \(IP: ([\d.]+) \| Online IDs:([^)|]+)\)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_CONNECTED event: %v", args)

			// Parse online IDs from the log
			idsString := args[5]
			ids := make(map[string]string)

			// Split IDs by commas and extract platform:id pairs
			idPairs := strings.Split(idsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					ids[platform] = id
				}
			}

			// Build player data
			player := map[string]interface{}{
				"playercontroller": args[3],
				"ip":               args[4],
			}

			// Add all IDs to player data
			for platform, id := range ids {
				player[platform] = id
			}

			// Get EOS ID if available, otherwise use Steam ID as fallback
			playerID := ""
			if eosID, ok := ids["eos"]; ok {
				playerID = eosID
			} else if steamID, ok := ids["steam"]; ok {
				playerID = steamID
			}

			if playerID != "" {
				// Store player data
				server.eventStore.mu.Lock()
				server.eventStore.joinRequests[args[2]] = player
				server.eventStore.players[playerID] = player

				// Handle reconnecting players
				delete(server.eventStore.disconnected, playerID)
				server.eventStore.mu.Unlock()
			}

			// Build event data
			eventData := map[string]interface{}{
				"raw":              args[0],
				"time":             args[1],
				"chainID":          args[2],
				"playercontroller": args[3],
				"ip":               args[4],
			}

			// Add all IDs to event data
			for platform, id := range ids {
				eventData[platform] = id
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_CONNECTED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},

	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadGameEvents: Display: Team ([0-9]), (.*) \( ?(.*?) ?\) has (won|lost) the match with ([0-9]+) Tickets on layer (.*) \(level (.*)\)!`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched NEW_GAME event (tickets): %v", args)

			// Build event data
			eventData := map[string]interface{}{
				"raw":        args[0],
				"time":       args[1],
				"chainID":    args[2],
				"team":       args[3],
				"subfaction": args[4],
				"faction":    args[5],
				"action":     args[6],
				"tickets":    args[7],
				"layer":      args[8],
				"level":      args[9],
			}

			// Store in event store based on win/loss status
			server.eventStore.mu.Lock()
			if args[6] == "won" {
				server.eventStore.session["ROUND_WINNER"] = eventData
			} else {
				server.eventStore.session["ROUND_LOSER"] = eventData
			}
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "NEW_GAME",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: UChannel::Close: Sending CloseBunch\. ChIndex == [0-9]+\. Name: \[UChannel\] ChIndex: [0-9]+, Closing: [0-9]+ \[UNetConnection\] RemoteAddr: ([\d.]+):[\d]+, Name: EOSIpNetConnection_[0-9]+, Driver: GameNetDriver EOSNetDriver_[0-9]+, IsServer: YES, PC: ([^ ]+PlayerController_C_[0-9]+), Owner: [^ ]+PlayerController_C_[0-9]+, UniqueId: RedpointEOS:([\d\w]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_DISCONNECTED event: %v", args)

			// Build event data
			eventData := map[string]interface{}{
				"raw":              args[0],
				"time":             args[1],
				"chainID":          args[2],
				"ip":               args[3],
				"playerController": args[4],
				"eosID":            args[5],
			}

			// Mark player as disconnected in the store
			eosID := args[5]
			server.eventStore.mu.Lock()
			if server.eventStore.disconnected == nil {
				server.eventStore.disconnected = make(map[string]map[string]interface{})
			}

			// Store the disconnection data
			server.eventStore.disconnected[eosID] = eventData
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_DISCONNECTED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: Player:(.+) ActualDamage=([0-9.]+) from (.+) \(Online IDs:([^|]+)\| Player Controller ID: ([^ ]+)\)caused by ([A-z_0-9-]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_DAMAGED event: %v", args)

			// Skip if IDs are invalid
			if strings.Contains(args[6], "INVALID") {
				return
			}

			// Parse online IDs from the log
			idsString := args[6]
			ids := make(map[string]string)

			// Split IDs by commas and extract platform:id pairs
			idPairs := strings.Split(idsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					ids[platform] = id
				}
			}

			// Build event data
			eventData := map[string]interface{}{
				"raw":                args[0],
				"time":               args[1],
				"chainID":            args[2],
				"victimName":         args[3],
				"damage":             args[4],
				"attackerName":       args[5],
				"attackerController": args[7],
				"weapon":             args[8],
			}

			// Add all attacker IDs to event data with capitalized platform name
			for platform, id := range ids {
				// Capitalize first letter of platform for key name
				platformKey := "attacker"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			// Store session data for the victim
			victimName := args[3]
			server.eventStore.mu.Lock()
			server.eventStore.session[victimName] = eventData

			// Update player data for attacker if EOS ID exists
			if eosID, ok := ids["eos"]; ok {
				// Initialize attacker data if it doesn't exist
				if _, exists := server.eventStore.players[eosID]; !exists {
					server.eventStore.players[eosID] = make(map[string]interface{})
				}
				server.eventStore.players[eosID]["controller"] = args[7]
			}
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_DAMAGED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Die\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-z_0-9]+) \(Online IDs:([^)|]+)\| Contoller ID: ([\w\d]+)\) caused by ([A-z_0-9-]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_DIED event: %v", args)

			// Skip if IDs are invalid
			if strings.Contains(args[6], "INVALID") {
				return
			}

			// Parse online IDs from the log
			idsString := args[6]
			ids := make(map[string]string)

			// Split IDs by commas and extract platform:id pairs
			idPairs := strings.Split(idsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					ids[platform] = id
				}
			}

			// Get existing session data for this victim
			victimName := args[3]
			server.eventStore.mu.RLock()
			var existingData map[string]interface{}
			if sessionData, exists := server.eventStore.session[victimName]; exists {
				// Make a copy of existing data
				existingData = make(map[string]interface{})
				for k, v := range sessionData {
					existingData[k] = v
				}
			} else {
				existingData = make(map[string]interface{})
			}
			server.eventStore.mu.RUnlock()

			// Build event data, merging with existing session data
			eventData := existingData
			eventData["raw"] = args[0]
			eventData["time"] = args[1]
			eventData["woundTime"] = args[1]
			eventData["chainID"] = args[2]
			eventData["victimName"] = args[3]
			eventData["damage"] = args[4]
			eventData["attackerPlayerController"] = args[5]
			eventData["weapon"] = args[8]

			// Add all attacker IDs to event data with capitalized platform name
			for platform, id := range ids {
				// Capitalize first letter of platform for key name
				platformKey := "attacker"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			// Get victim and attacker team information to check for teamkill
			var isTeamkill bool
			var victimTeamID, attackerTeamID string

			// Look up victim team ID
			server.eventStore.mu.RLock()
			if victimData, exists := server.eventStore.session[victimName]; exists {
				if teamID, hasTeam := victimData["teamID"].(string); hasTeam {
					victimTeamID = teamID
				}
			}

			// Look up attacker EOS ID from the event data
			var attackerEOSID string
			if eosID, hasEOS := eventData["attackerEos"].(string); hasEOS {
				attackerEOSID = eosID
			}

			// Look up attacker team ID if we have their EOS ID
			if attackerEOSID != "" {
				if attackerData, exists := server.eventStore.players[attackerEOSID]; exists {
					if teamID, hasTeam := attackerData["teamID"].(string); hasTeam {
						attackerTeamID = teamID
					}
				}
			}
			server.eventStore.mu.RUnlock()

			// Check for teamkill: same team but different players
			if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
				// Ensure this isn't self-damage
				var victimEOSID string
				server.eventStore.mu.RLock()
				if victimData, exists := server.eventStore.session[victimName]; exists {
					if eosID, hasEOS := victimData["eosID"].(string); hasEOS {
						victimEOSID = eosID
					}
				}
				server.eventStore.mu.RUnlock()

				// If we have both EOSIDs and they're different, but teams are the same, it's a teamkill
				if victimEOSID != "" && attackerEOSID != "" && victimEOSID != attackerEOSID {
					isTeamkill = true
					eventData["teamkill"] = true
				}
			}

			// Update session data
			server.eventStore.mu.Lock()
			server.eventStore.session[victimName] = eventData
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_DIED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)

			// If it's a teamkill, emit a separate TEAMKILL event
			if isTeamkill {
				teamkillData := &pb.EventEntry{
					Event: "TEAMKILL",
					Data:  string(jsonBytes),
				}
				server.broadcastEvent(teamkillData)
			}
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogNet: Join succeeded: (.+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched JOIN_SUCCEEDED event: %v", args)

			// Convert chainID to number (stored as string in Go)
			chainID := args[2]

			// Fetch player data by chainID
			server.eventStore.mu.Lock()
			player, exists := server.eventStore.joinRequests[chainID]
			if !exists {
				log.Error().Msgf("No join request found for chainID: %s", chainID)
				server.eventStore.mu.Unlock()
				return
			}

			// Join request processed, remove it
			delete(server.eventStore.joinRequests, chainID)

			// Create event data by combining player data with new data
			eventData := make(map[string]interface{})

			// Copy all player data to event data
			for k, v := range player {
				eventData[k] = v
			}

			// Add new fields
			eventData["raw"] = args[0]
			eventData["time"] = args[1]
			eventData["chainID"] = chainID
			eventData["playerSuffix"] = args[3]

			// Update player data with suffix
			player["playerSuffix"] = args[3]
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "JOIN_SUCCEEDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?OnPossess\(\): PC=(.+) \(Online IDs:([^)]+)\) Pawn=([A-z0-9_]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_POSSESS event: %v", args)

			// Parse online IDs from the log
			idsString := args[4]
			ids := make(map[string]string)

			// Split IDs by commas and extract platform:id pairs
			idPairs := strings.Split(idsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					ids[platform] = id
				}
			}

			// Build event data
			eventData := map[string]interface{}{
				"raw":              args[0],
				"time":             args[1],
				"chainID":          args[2],
				"playerSuffix":     args[3],
				"possessClassname": args[5],
			}

			// Add all player IDs to event data with capitalized platform name
			for platform, id := range ids {
				// Capitalize first letter of platform for key name
				platformKey := "player"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			// Store chainID in session data for the player suffix
			playerSuffix := args[3]
			server.eventStore.mu.Lock()
			server.eventStore.session[playerSuffix] = map[string]interface{}{
				"chainID": args[2],
			}
			server.eventStore.mu.Unlock()

			// Add deprecated field for compatibility
			if steamID, ok := ids["steam"]; ok {
				eventData["pawn"] = steamID
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_POSSESS",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: (.+) \(Online IDs:([^)]+)\) has revived (.+) \(Online IDs:([^)]+)\)\.`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_REVIVED event: %v", args)

			// Parse reviver IDs
			reviverIdsString := args[4]
			reviverIds := make(map[string]string)
			idPairs := strings.Split(reviverIdsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					reviverIds[platform] = id
				}
			}

			// Parse victim IDs
			victimIdsString := args[6]
			victimIds := make(map[string]string)
			idPairs = strings.Split(victimIdsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					victimIds[platform] = id
				}
			}

			// Get existing session data
			reviverName := args[3]
			server.eventStore.mu.RLock()
			var existingData map[string]interface{}
			if sessionData, exists := server.eventStore.session[reviverName]; exists {
				// Make a copy of existing data
				existingData = make(map[string]interface{})
				for k, v := range sessionData {
					existingData[k] = v
				}
			} else {
				existingData = make(map[string]interface{})
			}
			server.eventStore.mu.RUnlock()

			// Build event data, merging with existing session data
			eventData := existingData
			eventData["raw"] = args[0]
			eventData["time"] = args[1]
			eventData["chainID"] = args[2]
			eventData["reviverName"] = args[3]
			eventData["victimName"] = args[5]

			// Add all reviver IDs to event data with capitalized platform name
			for platform, id := range reviverIds {
				// Capitalize first letter of platform for key name
				platformKey := "reviver"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			// Add all victim IDs to event data with capitalized platform name
			for platform, id := range victimIds {
				// Capitalize first letter of platform for key name
				platformKey := "victim"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_REVIVED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQSoldier::)?Wound\(\): Player:(.+) KillingDamage=(?:-)*([0-9.]+) from ([A-z_0-9]+) \(Online IDs:([^)|]+)\| Controller ID: ([\w\d]+)\) caused by ([A-z_0-9-]+)_C`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_WOUNDED event: %v", args)

			// Skip if IDs are invalid
			if strings.Contains(args[6], "INVALID") {
				return
			}

			// Parse online IDs from the log
			idsString := args[6]
			ids := make(map[string]string)

			// Split IDs by commas and extract platform:id pairs
			idPairs := strings.Split(idsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					ids[platform] = id
				}
			}

			// Get existing session data for this victim
			victimName := args[3]
			server.eventStore.mu.RLock()
			var existingData map[string]interface{}
			if sessionData, exists := server.eventStore.session[victimName]; exists {
				// Make a copy of existing data
				existingData = make(map[string]interface{})
				for k, v := range sessionData {
					existingData[k] = v
				}
			} else {
				existingData = make(map[string]interface{})
			}
			server.eventStore.mu.RUnlock()

			// Build event data, merging with existing session data
			eventData := existingData
			eventData["raw"] = args[0]
			eventData["time"] = args[1]
			eventData["chainID"] = args[2]
			eventData["victimName"] = args[3]
			eventData["damage"] = args[4]
			eventData["attackerPlayerController"] = args[5]
			eventData["weapon"] = args[8]

			// Add all attacker IDs to event data with capitalized platform name
			for platform, id := range ids {
				// Capitalize first letter of platform for key name
				platformKey := "attacker"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			// Get victim and attacker team information to check for teamkill
			var isTeamkill bool
			var victimTeamID, attackerTeamID string

			// Look up victim team ID
			server.eventStore.mu.RLock()
			if victimData, exists := server.eventStore.session[victimName]; exists {
				if teamID, hasTeam := victimData["teamID"].(string); hasTeam {
					victimTeamID = teamID
				}
			}

			// Look up attacker EOS ID from the event data
			var attackerEOSID string
			if eosID, hasEOS := eventData["attackerEos"].(string); hasEOS {
				attackerEOSID = eosID
			}

			// Look up attacker team ID if we have their EOS ID
			if attackerEOSID != "" {
				if attackerData, exists := server.eventStore.players[attackerEOSID]; exists {
					if teamID, hasTeam := attackerData["teamID"].(string); hasTeam {
						attackerTeamID = teamID
					}
				}
			}
			server.eventStore.mu.RUnlock()

			// Check for teamkill: same team but different players
			if victimTeamID != "" && attackerTeamID != "" && victimTeamID == attackerTeamID {
				// Ensure this isn't self-damage
				var victimEOSID string
				server.eventStore.mu.RLock()
				if victimData, exists := server.eventStore.session[victimName]; exists {
					if eosID, hasEOS := victimData["eosID"].(string); hasEOS {
						victimEOSID = eosID
					}
				}
				server.eventStore.mu.RUnlock()

				// If we have both EOSIDs and they're different, but teams are the same, it's a teamkill
				if victimEOSID != "" && attackerEOSID != "" && victimEOSID != attackerEOSID {
					isTeamkill = true
					eventData["teamkill"] = true
				}
			}

			// Update session data
			server.eventStore.mu.Lock()
			server.eventStore.session[victimName] = eventData
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_WOUNDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)

			// If it's a teamkill, emit a separate TEAMKILL event
			if isTeamkill {
				teamkillData := &pb.EventEntry{
					Event: "TEAMKILL",
					Data:  string(jsonBytes),
				}
				server.broadcastEvent(teamkillData)
			}
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquad: USQGameState: Server Tick Rate: ([0-9.]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched TICK_RATE event: %v", args)
			// Build a JSON object with the event details.
			eventData := map[string]string{
				"time":     args[1],
				"chainID":  args[2],
				"tickRate": args[3],
			}

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "TICK_RATE",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQGameMode::)?DetermineMatchWinner\(\): (.+) won on (.+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched ROUND_ENDED event: %v", args)

			// Build event data
			eventData := map[string]interface{}{
				"raw":     args[0],
				"time":    args[1],
				"chainID": args[2],
				"winner":  args[3],
				"layer":   args[4],
			}

			// Store in event store
			server.eventStore.mu.Lock()
			// Check if WON already exists
			_, wonExists := server.eventStore.session["WON"]
			if wonExists {
				// If WON exists, store with null winner
				nullWinnerData := make(map[string]interface{})
				for k, v := range eventData {
					nullWinnerData[k] = v
				}
				nullWinnerData["winner"] = nil
				server.eventStore.session["WON"] = nullWinnerData
			} else {
				// Otherwise, store original data
				server.eventStore.session["WON"] = eventData
			}
			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "ROUND_ENDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogGameState: Match State Changed from InProgress to WaitingPostMatch`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched MATCH_STATE_CHANGE event (to WaitingPostMatch): %v", args)

			// Get winner and loser data from event store
			server.eventStore.mu.Lock()

			// Initialize event data with time
			eventData := map[string]interface{}{
				"time": args[1],
			}

			// Add winner data if it exists
			if winnerData, exists := server.eventStore.session["ROUND_WINNER"]; exists {
				eventData["winner"] = winnerData
			} else {
				eventData["winner"] = nil
			}

			// Add loser data if it exists
			if loserData, exists := server.eventStore.session["ROUND_LOSER"]; exists {
				eventData["loser"] = loserData
			} else {
				eventData["loser"] = nil
			}

			// Clean up event store
			delete(server.eventStore.session, "ROUND_WINNER")
			delete(server.eventStore.session, "ROUND_LOSER")

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "ROUND_ENDED",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogWorld: Bringing World \/([A-z]+)\/(?:Maps\/)?([A-z0-9-]+)\/(?:.+\/)?([A-z0-9-]+)(?:\.[A-z0-9-]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched NEW_GAME event (map loading): %v", args)

			// Skip transition map
			if args[5] == "TransitionMap" {
				return
			}

			// Get WON data from event store if it exists
			server.eventStore.mu.Lock()

			// Initialize event data
			eventData := map[string]interface{}{
				"raw":            args[0],
				"time":           args[1],
				"chainID":        args[2],
				"dlc":            args[3],
				"mapClassname":   args[4],
				"layerClassname": args[5],
			}

			// Merge with WON data if it exists
			if wonData, exists := server.eventStore.session["WON"]; exists {
				for k, v := range wonData {
					eventData[k] = v
				}
				// Clean up WON data
				delete(server.eventStore.session, "WON")
			}

			// Clear the event store for the new game
			server.eventStore.session = make(map[string]map[string]interface{})
			server.eventStore.disconnected = make(map[string]map[string]interface{})
			// Note: We don't clear players or joinRequests as they persist across map changes

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "NEW_GAME",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?SquadJoined\(\): Player:(.+) \(Online IDs:([^)]+)\) Joined Team ([0-9]) Squad ([0-9]+)`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_SQUAD_CHANGE event: %v", args)

			// Parse online IDs from the log
			idsString := args[4]
			ids := make(map[string]string)

			// Split IDs by commas and extract platform:id pairs
			idPairs := strings.Split(idsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					ids[platform] = id
				}
			}

			// Build event data
			playerName := args[3]
			teamID := args[5]
			squadID := args[6]

			eventData := map[string]interface{}{
				"raw":        args[0],
				"time":       args[1],
				"chainID":    args[2],
				"name":       playerName,
				"teamID":     teamID,
				"squadID":    squadID,
				"oldTeamID":  nil, // We don't track previous team in this implementation
				"oldSquadID": nil, // We don't track previous squad in this implementation
			}

			// Add all player IDs to event data with capitalized platform name
			for platform, id := range ids {
				// Capitalize first letter of platform for key name
				platformKey := "player"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			// Store player information including team ID in session data
			server.eventStore.mu.Lock()

			// Create/update player session data
			if _, exists := server.eventStore.session[playerName]; !exists {
				server.eventStore.session[playerName] = make(map[string]interface{})
			}
			server.eventStore.session[playerName]["teamID"] = teamID
			server.eventStore.session[playerName]["squadID"] = squadID

			// If we have an EOS ID, also store team info by EOS ID
			if eosID, ok := ids["eos"]; ok {
				if _, exists := server.eventStore.players[eosID]; !exists {
					server.eventStore.players[eosID] = make(map[string]interface{})
				}
				server.eventStore.players[eosID]["teamID"] = teamID
				server.eventStore.players[eosID]["squadID"] = squadID
				server.eventStore.players[eosID]["name"] = playerName
			}

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_SQUAD_CHANGE",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
	{
		regex: regexp.MustCompile(`^\[([0-9.:-]+)]\[([ 0-9]*)]LogSquadTrace: \[DedicatedServer](?:ASQPlayerController::)?TeamJoined\(\): Player:(.+) \(Online IDs:([^)]+)\) Is Now On Team ([0-9])`),
		onMatch: func(args []string, server *LogWatcherServer) {
			log.Info().Msgf("Matched PLAYER_TEAM_CHANGE event: %v", args)

			// Parse online IDs from the log
			idsString := args[4]
			ids := make(map[string]string)

			// Split IDs by commas and extract platform:id pairs
			idPairs := strings.Split(idsString, ",")
			for _, pair := range idPairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, ":")
				if len(parts) == 2 {
					platform := strings.ToLower(strings.TrimSpace(parts[0]))
					id := strings.TrimSpace(parts[1])
					ids[platform] = id
				}
			}

			// Build event data
			playerName := args[3]
			newTeamID := args[5]

			// Get old team ID if available
			var oldTeamID interface{} = nil
			server.eventStore.mu.RLock()
			if playerData, exists := server.eventStore.session[playerName]; exists {
				if teamID, hasTeam := playerData["teamID"]; hasTeam {
					oldTeamID = teamID
				}
			}
			server.eventStore.mu.RUnlock()

			eventData := map[string]interface{}{
				"raw":       args[0],
				"time":      args[1],
				"chainID":   args[2],
				"name":      playerName,
				"newTeamID": newTeamID,
				"oldTeamID": oldTeamID,
			}

			// Add all player IDs to event data with capitalized platform name
			for platform, id := range ids {
				// Capitalize first letter of platform for key name
				platformKey := "player"
				if len(platform) > 0 {
					platformKey += strings.ToUpper(platform[:1])
					if len(platform) > 1 {
						platformKey += platform[1:]
					}
				}
				eventData[platformKey] = id
			}

			// Store player information including team ID in session data
			server.eventStore.mu.Lock()

			// Create/update player session data
			if _, exists := server.eventStore.session[playerName]; !exists {
				server.eventStore.session[playerName] = make(map[string]interface{})
			}
			server.eventStore.session[playerName]["teamID"] = newTeamID

			// If we have an EOS ID, also store team info by EOS ID
			if eosID, ok := ids["eos"]; ok {
				if _, exists := server.eventStore.players[eosID]; !exists {
					server.eventStore.players[eosID] = make(map[string]interface{})
				}
				server.eventStore.players[eosID]["teamID"] = newTeamID
				server.eventStore.players[eosID]["name"] = playerName
			}

			server.eventStore.mu.Unlock()

			jsonBytes, err := json.Marshal(eventData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal event data")
				return
			}

			data := &pb.EventEntry{
				Event: "PLAYER_TEAM_CHANGE",
				Data:  string(jsonBytes),
			}
			server.broadcastEvent(data)
		},
	},
}

// NewLogWatcherServer initializes the server
func NewLogWatcherServer(logSource LogSource) *LogWatcherServer {
	ctx, cancel := context.WithCancel(context.Background())
	server := &LogWatcherServer{
		clients:    make(map[pb.LogWatcher_StreamLogsServer]struct{}),
		eventSubs:  make(map[pb.LogWatcher_StreamEventsServer]struct{}),
		logSource:  logSource,
		eventStore: NewEventStore(),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start processing logs for events
	go server.processLogs()

	return server
}

// Authenticate using a simple token
func validateToken(tokenString string) bool {
	if tokenString == authToken {
		log.Info().Msg("Authentication successful")
		return true
	}
	log.Info().Msg("Authentication failed")
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

	// Keep stream open
	<-stream.Context().Done()
	return nil
}

// StreamEvents streams structured events found in logs
func (s *LogWatcherServer) StreamEvents(req *pb.AuthRequest, stream pb.LogWatcher_StreamEventsServer) error {
	if !validateToken(req.Token) {
		return fmt.Errorf("unauthorized")
	}

	log.Info().Msg("New StreamEvents subscriber")

	s.mu.Lock()
	s.eventSubs[stream] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.eventSubs, stream)
		s.mu.Unlock()
	}()

	// Keep stream open
	<-stream.Context().Done()
	return nil
}

// processLogs continuously reads logs and processes events
func (s *LogWatcherServer) processLogs() {
	logChan, err := s.logSource.Watch(s.ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to watch logs")
	}

	for logLine := range logChan {
		s.processLogForEvents(logLine)
		s.broadcastLogLine(logLine)
	}
}

// broadcastLogLine sends log line to all connected log stream clients
func (s *LogWatcherServer) broadcastLogLine(logLine string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for stream := range s.clients {
		stream.Send(&pb.LogEntry{Content: logLine})
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

// Shutdown gracefully shuts down the server
func (s *LogWatcherServer) Shutdown() {
	s.cancel()
	if s.logSource != nil {
		s.logSource.Close()
	}
}

// StartServer runs the gRPC server
func StartServer(ctx context.Context, c *cli.Command) error {
	port := c.String("port")
	authToken = c.String("auth-token")
	sourceType := c.String("source-type")
	pollFrequency := c.Duration("poll-frequency")
	readFromStart := c.Bool("read-from-start")
	logLevel := c.String("log-level")
	logPretty := c.Bool("log-pretty")
	logNoColor := c.Bool("log-no-color")

	err := logger.SetupGlobalLogger(ctx, logLevel, logPretty, logNoColor, "", true)
	if err != nil {
		return fmt.Errorf("failed to set up logger: %v", err)
	}

	var logSource LogSource

	switch sourceType {
	case "local":
		logFile := c.String("log-file")
		if logFile == "" {
			return fmt.Errorf("log-file must be specified for local source type")
		}
		logSource = NewLocalFileSource(logFile)
		log.Info().Msgf("Using local file source: %s", logFile)

	case "sftp":
		host := c.String("host")
		if host == "" {
			return fmt.Errorf("host must be specified for sftp source type")
		}
		remotePort := c.Int("remote-port")
		if remotePort == 0 {
			remotePort = 22 // Default SFTP port
		}
		username := c.String("username")
		if username == "" {
			return fmt.Errorf("username must be specified for sftp source type")
		}
		password := c.String("password")
		keyPath := c.String("key-path")
		if password == "" && keyPath == "" {
			return fmt.Errorf("either password or key-path must be specified for sftp source type")
		}
		remotePath := c.String("remote-path")
		if remotePath == "" {
			return fmt.Errorf("remote-path must be specified for sftp source type")
		}
		logSource = NewSFTPSource(host, int(remotePort), username, password, keyPath, remotePath, pollFrequency, readFromStart)
		log.Info().Msgf("Using SFTP source: %s@%s:%d%s (read from start: %v)", username, host, remotePort, remotePath, readFromStart)

	case "ftp":
		host := c.String("host")
		if host == "" {
			return fmt.Errorf("host must be specified for ftp source type")
		}
		remotePort := c.Int("remote-port")
		if remotePort == 0 {
			remotePort = 21 // Default FTP port
		}
		username := c.String("username")
		if username == "" {
			return fmt.Errorf("username must be specified for ftp source type")
		}
		password := c.String("password")
		if password == "" {
			return fmt.Errorf("password must be specified for ftp source type")
		}
		remotePath := c.String("remote-path")
		if remotePath == "" {
			return fmt.Errorf("remote-path must be specified for ftp source type")
		}
		logSource = NewFTPSource(host, int(remotePort), username, password, remotePath, pollFrequency, readFromStart)
		log.Info().Msgf("Using FTP source: %s@%s:%d%s (read from start: %v)", username, host, remotePort, remotePath, readFromStart)

	default:
		return fmt.Errorf("invalid source-type: %s, must be 'local', 'sftp', or 'ftp'", sourceType)
	}

	server := NewLogWatcherServer(logSource)
	defer server.Shutdown()

	// Start gRPC server
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to listen")
	}
	grpcServer := grpc.NewServer()
	pb.RegisterLogWatcherServer(grpcServer, server)

	log.Info().Msgf("LogWatcher gRPC server listening on :%s", port)

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		log.Info().Msg("Shutting down server...")
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(lis)
}

func main() {
	ctx := utils.WithContextSigtermCallback(context.Background(), func() {
		log.Info().Msg("Received SIGTERM, shutting down")
	})

	app := &cli.Command{
		Name:  "logwatcher",
		Usage: "Watches a file and streams changes via gRPC",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_SOURCE_TYPE"),
				Name:     "source-type",
				Usage:    "Type of log source (local, sftp, ftp)",
				Value:    "local",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_LOG_FILE"),
				Name:     "log-file",
				Usage:    "Path to the local log file to watch (for local source type)",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_HOST"),
				Name:     "host",
				Usage:    "Remote host for SFTP/FTP connection",
				Required: false,
			},
			&cli.IntFlag{
				Sources:  cli.EnvVars("LOGWATCHER_REMOTE_PORT"),
				Name:     "remote-port",
				Usage:    "Port for SFTP/FTP connection",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_USERNAME"),
				Name:     "username",
				Usage:    "Username for SFTP/FTP connection",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_PASSWORD"),
				Name:     "password",
				Usage:    "Password for SFTP/FTP connection",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_KEY_PATH"),
				Name:     "key-path",
				Usage:    "Path to private key file for SFTP authentication",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_REMOTE_PATH"),
				Name:     "remote-path",
				Usage:    "Path to the remote log file for SFTP/FTP",
				Required: false,
			},
			&cli.DurationFlag{
				Sources:  cli.EnvVars("LOGWATCHER_POLL_FREQUENCY"),
				Name:     "poll-frequency",
				Usage:    "How often to poll for changes in SFTP/FTP mode",
				Value:    5 * time.Second,
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_PORT"),
				Name:     "port",
				Usage:    "Port to run the gRPC server on",
				Value:    "31135",
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_AUTH_TOKEN"),
				Name:     "auth-token",
				Usage:    "Simple auth token for authentication",
				Required: true,
			},
			&cli.BoolFlag{
				Sources:  cli.EnvVars("LOGWATCHER_READ_FROM_START"),
				Name:     "read-from-start",
				Usage:    "Read the entire log file from the beginning rather than tailing only new entries",
				Value:    false,
				Required: false,
			},
			&cli.StringFlag{
				Sources:  cli.EnvVars("LOGWATCHER_LOG_LEVEL"),
				Name:     "log-level",
				Usage:    "Log level (debug, info, warn, error, fatal, panic)",
				Value:    "info",
				Required: false,
			},
			&cli.BoolFlag{
				Sources:  cli.EnvVars("LOGWATCHER_LOG_PRETTY"),
				Name:     "log-pretty",
				Usage:    "Enable pretty logging",
				Value:    true,
				Required: false,
			},
			&cli.BoolFlag{
				Sources:  cli.EnvVars("LOGWATCHER_LOG_NO_COLOR"),
				Name:     "log-no-color",
				Usage:    "Disable color output",
				Value:    false,
				Required: false,
			},
		},
		Action: StartServer,
	}

	if err := app.Run(ctx, os.Args); err != nil {
		log.Error().Msgf("error running Squad Aegis log watcher: %v", err)
	}
}
