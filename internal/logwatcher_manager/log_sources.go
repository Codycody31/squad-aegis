package logwatcher_manager

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// LogSourceType represents the type of log source
type LogSourceType string

const (
	LogSourceTypeLocal LogSourceType = "local"
	LogSourceTypeSFTP  LogSourceType = "sftp"
	LogSourceTypeFTP   LogSourceType = "ftp"
)

// LogSource defines an interface for different log sources
type LogSource interface {
	// Watch starts watching logs and returns a channel that receives log lines
	Watch(ctx context.Context) (<-chan string, error)
	// Close the log source
	Close() error
}

// LogSourceConfig represents configuration for a log source
type LogSourceConfig struct {
	Type          LogSourceType `json:"type"`
	FilePath      string        `json:"file_path"`
	Host          string        `json:"host,omitempty"`
	Port          int           `json:"port,omitempty"`
	Username      string        `json:"username,omitempty"`
	Password      string        `json:"password,omitempty"`
	PollFrequency time.Duration `json:"poll_frequency,omitempty"`
	ReadFromStart bool          `json:"read_from_start,omitempty"`
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
	filepath       string
	client         *sftp.Client
	sshConn        *ssh.Client
	pollFreq       time.Duration
	lastPos        int64
	mu             sync.Mutex
	reconnectDelay time.Duration
	maxDelay       time.Duration
	tempFilePath   string
	readFromStart  bool
}

// NewSFTPSource creates a new SFTP source
func NewSFTPSource(host string, port int, username, password, filepath string, pollFreq time.Duration, readFromStart bool) *SFTPSource {
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
			log.Error().Err(err).Msg("Failed to create temp file")
		} else {
			file.Close()
		}
	}

	return &SFTPSource{
		host:           host,
		port:           port,
		username:       username,
		password:       password,
		filepath:       filepath,
		pollFreq:       pollFreq,
		lastPos:        0,
		reconnectDelay: 1 * time.Second,
		maxDelay:       60 * time.Second,
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
					log.Warn().Msg("SFTP connection test failed, attempting reconnect")
					if err := s.reconnect(); err != nil {
						log.Error().Err(err).Msg("Failed to reconnect to SFTP server")
						continue
					}
				}

				// Download new data and get lines
				newLines, err := s.fetchNewData()
				if err != nil {
					log.Error().Err(err).Msg("Failed to fetch data from SFTP")
					// Try to reconnect on error
					if err := s.reconnect(); err != nil {
						log.Error().Err(err).Msg("Failed to reconnect to SFTP server")
					}
					continue
				}

				// Reset reconnect delay after successful fetch
				s.mu.Lock()
				s.reconnectDelay = 1 * time.Second
				s.mu.Unlock()

				// Send each line to the channel
				for _, line := range newLines {
					select {
					case logChan <- line:
					case <-ctx.Done():
						return
					}
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
		log.Info().Msg("Initial SFTP position set to start of file (reading from beginning)")
	} else {
		s.lastPos = fileSize
		log.Info().Int64("bytes", fileSize).Msg("Initial SFTP position set to end of file")
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
		log.Info().
			Int64("oldSize", s.lastPos).
			Int64("newSize", fileSize).
			Msg("File size decreased, file may have been rotated")
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

	log.Info().
		Str("host", s.host).
		Int("port", s.port).
		Dur("delay", delay).
		Msg("Attempting to reconnect to SFTP server")

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
		return fmt.Errorf("%s", strings.Join(errMsgs, "; "))
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
	mu            sync.Mutex
	tempFilePath  string
	maxRetries    int
	retryDelay    time.Duration
	readFromStart bool
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
			log.Error().Err(err).Msg("Failed to create temp file")
		} else {
			file.Close()
		}
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
					log.Error().Err(err).Msg("Failed to fetch data from FTP")
					// Try to reconnect if connection was lost
					if err := f.reconnect(); err != nil {
						log.Error().Err(err).Msg("Failed to reconnect to FTP server")
					}
					continue
				}

				// Send each line to the channel
				for _, line := range newLines {
					select {
					case logChan <- line:
					case <-ctx.Done():
						return
					}
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
			log.Warn().
				Err(err).
				Int("attempt", retry+1).
				Int("maxRetries", f.maxRetries).
				Msg("Failed to get initial file size, retrying...")
			time.Sleep(f.retryDelay)

			// Try reconnecting before retry
			if strings.Contains(err.Error(), "connection") {
				f.reconnect()
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get initial file size after %d attempts: %v", f.maxRetries, err)
	}

	// Set initial position based on configuration
	if f.readFromStart {
		f.lastPos = 0
		log.Info().Msg("Initial FTP position set to start of file (reading from beginning)")
	} else {
		f.lastPos = fileSize
		log.Info().Int64("bytes", fileSize).Msg("Initial FTP position set to end of file")
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
			log.Warn().
				Err(err).
				Int("attempt", retry+1).
				Int("maxRetries", f.maxRetries).
				Msg("Failed to get file size, retrying...")
			time.Sleep(f.retryDelay)

			// Try reconnecting before retry
			if strings.Contains(err.Error(), "connection") {
				f.reconnect()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get file size after %d attempts: %v", f.maxRetries, err)
	}

	// If file size has not changed, return empty slice
	if f.lastPos == fileSize {
		return []string{}, nil
	}

	// If file has been rotated (smaller than our last position), reset position
	if f.lastPos > fileSize {
		log.Info().
			Int64("oldSize", f.lastPos).
			Int64("newSize", fileSize).
			Msg("File size decreased, file may have been rotated")
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
			log.Warn().
				Err(err).
				Int("attempt", retry+1).
				Int("maxRetries", f.maxRetries).
				Msg("Failed to retrieve file, retrying...")
			time.Sleep(f.retryDelay)

			// Try reconnecting before retry
			if strings.Contains(err.Error(), "connection") {
				f.reconnect()
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve file after %d attempts: %v", f.maxRetries, err)
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
	log.Info().
		Str("host", f.host).
		Int("port", f.port).
		Msg("Attempting to reconnect to FTP server")
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
		return fmt.Errorf("%s", strings.Join(errMsgs, "; "))
	}
	return nil
}
