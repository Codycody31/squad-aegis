package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.codycody31.dev/squad-aegis/internal/file_upload"
)

type motdUploadStub struct {
	atomicErr      error
	uploadErr      error
	atomicCalls    int
	uploadCalls    int
	atomicContent  []string
	regularContent []string
}

func (s *motdUploadStub) Upload(ctx context.Context, content string) error {
	s.uploadCalls++
	s.regularContent = append(s.regularContent, content)
	return s.uploadErr
}

func (s *motdUploadStub) UploadAtomically(ctx context.Context, content string) error {
	s.atomicCalls++
	s.atomicContent = append(s.atomicContent, content)
	return s.atomicErr
}

func (s *motdUploadStub) Read(ctx context.Context) (string, error) {
	return "", nil
}

func (s *motdUploadStub) TestConnection(ctx context.Context) error {
	return nil
}

func (s *motdUploadStub) Close() error {
	return nil
}

func TestUploadMOTDContentFallsBackWhenAtomicReplaceUnsupported(t *testing.T) {
	t.Parallel()

	uploader := &motdUploadStub{
		atomicErr: fmt.Errorf("rename blocked by server: %w", file_upload.ErrAtomicReplaceUnsupported),
	}

	const content = "Welcome to the server"

	if err := uploadMOTDContent(context.Background(), uploader, content); err != nil {
		t.Fatalf("uploadMOTDContent() error = %v, want nil", err)
	}
	if uploader.atomicCalls != 1 {
		t.Fatalf("atomicCalls = %d, want 1", uploader.atomicCalls)
	}
	if uploader.uploadCalls != 1 {
		t.Fatalf("uploadCalls = %d, want 1", uploader.uploadCalls)
	}
	if got := uploader.atomicContent[0]; got != content {
		t.Fatalf("atomic content = %q, want %q", got, content)
	}
	if got := uploader.regularContent[0]; got != content {
		t.Fatalf("fallback content = %q, want %q", got, content)
	}
}

func TestUploadMOTDContentDoesNotFallbackOnGenericAtomicError(t *testing.T) {
	t.Parallel()

	atomicErr := errors.New("temp upload failed")
	uploader := &motdUploadStub{atomicErr: atomicErr}

	err := uploadMOTDContent(context.Background(), uploader, "content")
	if !errors.Is(err, atomicErr) {
		t.Fatalf("uploadMOTDContent() error = %v, want %v", err, atomicErr)
	}
	if uploader.atomicCalls != 1 {
		t.Fatalf("atomicCalls = %d, want 1", uploader.atomicCalls)
	}
	if uploader.uploadCalls != 0 {
		t.Fatalf("uploadCalls = %d, want 0", uploader.uploadCalls)
	}
}

func TestUploadMOTDContentReturnsFallbackFailure(t *testing.T) {
	t.Parallel()

	fallbackErr := errors.New("stor denied")
	uploader := &motdUploadStub{
		atomicErr: fmt.Errorf("rename blocked by server: %w", file_upload.ErrAtomicReplaceUnsupported),
		uploadErr: fallbackErr,
	}

	err := uploadMOTDContent(context.Background(), uploader, "content")
	if !errors.Is(err, file_upload.ErrAtomicReplaceUnsupported) {
		t.Fatalf("uploadMOTDContent() error = %v, want atomic compatibility error", err)
	}
	if !errors.Is(err, fallbackErr) {
		t.Fatalf("uploadMOTDContent() error = %v, want fallback error", err)
	}
	if uploader.uploadCalls != 1 {
		t.Fatalf("uploadCalls = %d, want 1", uploader.uploadCalls)
	}
}
