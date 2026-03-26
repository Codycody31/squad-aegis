package file_upload

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var windowsDriveAbsPathPattern = regexp.MustCompile(`^[a-zA-Z]:/`)

// MaxReadBytes is the maximum number of bytes allowed when reading a remote file
// to prevent unbounded memory allocation from maliciously large files.
const MaxReadBytes = 10 * 1024 * 1024 // 10 MB

// validateFilePath rejects paths that contain traversal sequences or are not absolute.
func validateFilePath(p string) error {
	normalizedPath := strings.ReplaceAll(p, `\`, "/")
	if hasTraversalSegment(normalizedPath) {
		return fmt.Errorf("file path must not contain '..' traversal: %s", p)
	}

	cleaned := path.Clean(normalizedPath)
	if !isAbsoluteRemotePath(cleaned) {
		return fmt.Errorf("file path must be absolute: %s", p)
	}
	return nil
}

func hasTraversalSegment(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func isAbsoluteRemotePath(p string) bool {
	if filepath.IsAbs(p) {
		return true
	}

	// Support Windows absolute paths for remote hosts regardless of local OS,
	// e.g. D:/SquadGame/ServerConfig/Bans.cfg.
	return windowsDriveAbsPathPattern.MatchString(p)
}

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

func stagedRemotePath(targetPath string, kind string) string {
	return path.Join(
		path.Dir(targetPath),
		fmt.Sprintf(".%s.aegis-%s-%d", path.Base(targetPath), kind, time.Now().UnixNano()),
	)
}

// NewUploader creates the appropriate uploader based on protocol
func NewUploader(config UploadConfig) (Uploader, error) {
	if err := validateFilePath(config.FilePath); err != nil {
		return nil, err
	}
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
	return u.uploadToPath(u.config.FilePath, content)
}

func (u *SFTPUploader) uploadToPath(filePath string, content string) error {
	if u.sftpClient == nil {
		return fmt.Errorf("SFTP client not connected")
	}

	file, err := u.sftpClient.Create(filePath)
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

// UploadAtomically uploads content to a temporary file and then renames it into
// place so transient write failures do not truncate the live file first.
func (u *SFTPUploader) UploadAtomically(ctx context.Context, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := stagedRemotePath(u.config.FilePath, "tmp")
	if err := u.uploadToPath(tempPath, content); err != nil {
		_ = u.sftpClient.Remove(tempPath)
		return err
	}

	if err := ctx.Err(); err != nil {
		_ = u.sftpClient.Remove(tempPath)
		return err
	}

	if err := u.sftpClient.PosixRename(tempPath, u.config.FilePath); err == nil {
		return nil
	} else {
		if renameErr := u.sftpClient.Rename(tempPath, u.config.FilePath); renameErr == nil {
			return nil
		} else {
			backupPath := stagedRemotePath(u.config.FilePath, "bak")
			if backupErr := u.sftpClient.Rename(u.config.FilePath, backupPath); backupErr != nil {
				_ = u.sftpClient.Remove(tempPath)
				return fmt.Errorf("failed to replace remote file: posix-rename: %v; rename: %v; backup current: %v", err, renameErr, backupErr)
			}
			if promoteErr := u.sftpClient.Rename(tempPath, u.config.FilePath); promoteErr != nil {
				restoreErr := u.sftpClient.Rename(backupPath, u.config.FilePath)
				_ = u.sftpClient.Remove(tempPath)
				if restoreErr != nil {
					return fmt.Errorf("failed to promote temp remote file and failed to restore original: replace: %v; restore: %v", promoteErr, restoreErr)
				}
				return fmt.Errorf("failed to promote temp remote file: %v", promoteErr)
			}
			_ = u.sftpClient.Remove(backupPath)
			return nil
		}
	}
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
	limited := io.LimitReader(file, MaxReadBytes+1)
	if _, err := io.Copy(&buf, limited); err != nil {
		return "", fmt.Errorf("failed to read remote file: %v", err)
	}
	if buf.Len() > MaxReadBytes {
		return "", fmt.Errorf("remote file exceeds maximum allowed size (%d bytes)", MaxReadBytes)
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
	return u.uploadToPath(u.config.FilePath, content)
}

func (u *FTPUploader) uploadToPath(filePath string, content string) error {
	if u.conn == nil {
		return fmt.Errorf("FTP connection not established")
	}

	err := u.conn.Stor(filePath, strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	return nil
}

// UploadAtomically uploads content to a temporary file and then renames it into
// place so transient write failures do not truncate the live file first.
func (u *FTPUploader) UploadAtomically(ctx context.Context, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := stagedRemotePath(u.config.FilePath, "tmp")
	if err := u.uploadToPath(tempPath, content); err != nil {
		_ = u.conn.Delete(tempPath)
		return err
	}

	if err := ctx.Err(); err != nil {
		_ = u.conn.Delete(tempPath)
		return err
	}

	if err := u.conn.Rename(tempPath, u.config.FilePath); err == nil {
		return nil
	} else {
		backupPath := stagedRemotePath(u.config.FilePath, "bak")
		if backupErr := u.conn.Rename(u.config.FilePath, backupPath); backupErr != nil {
			_ = u.conn.Delete(tempPath)
			return fmt.Errorf("failed to replace remote file: rename temp into place: %v; backup current: %v", err, backupErr)
		}
		if promoteErr := u.conn.Rename(tempPath, u.config.FilePath); promoteErr != nil {
			restoreErr := u.conn.Rename(backupPath, u.config.FilePath)
			_ = u.conn.Delete(tempPath)
			if restoreErr != nil {
				return fmt.Errorf("failed to promote temp remote file and failed to restore original: replace: %v; restore: %v", promoteErr, restoreErr)
			}
			return fmt.Errorf("failed to promote temp remote file: %v", promoteErr)
		}
		_ = u.conn.Delete(backupPath)
		return nil
	}
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
	limited := io.LimitReader(resp, MaxReadBytes+1)
	if _, err := io.Copy(&buf, limited); err != nil {
		return "", fmt.Errorf("failed to read remote file: %v", err)
	}
	if buf.Len() > MaxReadBytes {
		return "", fmt.Errorf("remote file exceeds maximum allowed size (%d bytes)", MaxReadBytes)
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
