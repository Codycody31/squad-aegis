package file_upload

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// UploadConfig holds configuration for file upload
type UploadConfig struct {
	Protocol string // "sftp" or "ftp"
	Host     string
	Port     int
	Username string
	Password string
	FilePath string
}

// Uploader interface for file upload implementations
type Uploader interface {
	Upload(ctx context.Context, content string) error
	Read(ctx context.Context) (string, error)
	TestConnection(ctx context.Context) error
	Close() error
}

// NewUploader creates the appropriate uploader based on protocol
func NewUploader(config UploadConfig) (Uploader, error) {
	switch config.Protocol {
	case "sftp":
		return NewSFTPUploader(config)
	case "ftp":
		return NewFTPUploader(config)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}
}

// SFTPUploader implements file upload via SFTP
type SFTPUploader struct {
	config     UploadConfig
	sshConn    *ssh.Client
	sftpClient *sftp.Client
}

// NewSFTPUploader creates a new SFTP uploader
func NewSFTPUploader(config UploadConfig) (*SFTPUploader, error) {
	uploader := &SFTPUploader{
		config: config,
	}

	if err := uploader.connect(); err != nil {
		return nil, err
	}

	return uploader, nil
}

func (u *SFTPUploader) connect() error {
	sshConfig := &ssh.ClientConfig{
		User:            u.config.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(u.config.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	sshConn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", u.config.Host, u.config.Port), sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SFTP server: %v", err)
	}

	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}

	u.sshConn = sshConn
	u.sftpClient = sftpClient

	return nil
}

// Upload uploads content to the remote file
func (u *SFTPUploader) Upload(ctx context.Context, content string) error {
	if u.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
	}

	// Create or overwrite the remote file
	file, err := u.sftpClient.Create(u.config.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to write content: %v", err)
	}

	return nil
}

// Read reads the content of the remote file
func (u *SFTPUploader) Read(ctx context.Context) (string, error) {
	if u.sftpClient == nil {
		return "", fmt.Errorf("SFTP client not connected")
	}

	file, err := u.sftpClient.Open(u.config.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open remote file: %v", err)
	}
	defer file.Close()

	var buf strings.Builder
	if _, err := io.Copy(&buf, file); err != nil {
		return "", fmt.Errorf("failed to read remote file: %v", err)
	}

	return buf.String(), nil
}

// TestConnection verifies the connection is working
func (u *SFTPUploader) TestConnection(ctx context.Context) error {
	if u.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
	}

	// Try to stat the parent directory to verify connection
	_, err := u.sftpClient.Stat("/")
	if err != nil {
		return fmt.Errorf("connection test failed: %v", err)
	}

	return nil
}

// Close closes the SFTP connection
func (u *SFTPUploader) Close() error {
	var errs []string

	if u.sftpClient != nil {
		if err := u.sftpClient.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("failed to close SFTP client: %v", err))
		}
		u.sftpClient = nil
	}

	if u.sshConn != nil {
		if err := u.sshConn.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("failed to close SSH connection: %v", err))
		}
		u.sshConn = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// FTPUploader implements file upload via FTP
type FTPUploader struct {
	config UploadConfig
	conn   *ftp.ServerConn
}

// NewFTPUploader creates a new FTP uploader
func NewFTPUploader(config UploadConfig) (*FTPUploader, error) {
	uploader := &FTPUploader{
		config: config,
	}

	if err := uploader.connect(); err != nil {
		return nil, err
	}

	return uploader, nil
}

func (u *FTPUploader) connect() error {
	addr := fmt.Sprintf("%s:%d", u.config.Host, u.config.Port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("failed to connect to FTP server: %v", err)
	}

	if err := conn.Login(u.config.Username, u.config.Password); err != nil {
		conn.Quit()
		return fmt.Errorf("failed to login to FTP server: %v", err)
	}

	u.conn = conn
	return nil
}

// Upload uploads content to the remote file
func (u *FTPUploader) Upload(ctx context.Context, content string) error {
	if u.conn == nil {
		return fmt.Errorf("FTP connection not established")
	}

	err := u.conn.Stor(u.config.FilePath, strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	return nil
}

// Read reads the content of the remote file
func (u *FTPUploader) Read(ctx context.Context) (string, error) {
	if u.conn == nil {
		return "", fmt.Errorf("FTP connection not established")
	}

	resp, err := u.conn.Retr(u.config.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve remote file: %v", err)
	}
	defer resp.Close()

	var buf strings.Builder
	if _, err := io.Copy(&buf, resp); err != nil {
		return "", fmt.Errorf("failed to read remote file: %v", err)
	}

	return buf.String(), nil
}

// TestConnection verifies the connection is working
func (u *FTPUploader) TestConnection(ctx context.Context) error {
	if u.conn == nil {
		return fmt.Errorf("FTP connection not established")
	}

	// Try to get current directory to verify connection
	_, err := u.conn.CurrentDir()
	if err != nil {
		return fmt.Errorf("connection test failed: %v", err)
	}

	return nil
}

// Close closes the FTP connection
func (u *FTPUploader) Close() error {
	if u.conn != nil {
		err := u.conn.Quit()
		u.conn = nil
		return err
	}
	return nil
}
