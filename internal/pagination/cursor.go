package pagination

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	CursorVersion       = 1
	MaxEncodedCursorLen = 4096
	filterHashBytes     = 16
)

var (
	ErrInvalidCursor       = errors.New("invalid cursor")
	ErrCursorTooLarge      = errors.New("cursor is too large")
	ErrUnsupportedVersion  = errors.New("unsupported cursor version")
	ErrCursorKindMismatch  = errors.New("cursor endpoint does not match request")
	ErrCursorFilterChanged = errors.New("cursor filters do not match request")
)

// PageInfo describes the next page of a keyset-paginated response. NextCursor
// is nil when HasMore is false.
type PageInfo struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// CursorEnvelope is the versioned, endpoint-bound wire representation of an
// opaque cursor. Values contains the final row's stable sort tuple.
type CursorEnvelope struct {
	Version    int             `json:"v"`
	Kind       string          `json:"kind"`
	FilterHash string          `json:"filter_hash"`
	Values     json.RawMessage `json:"values"`
}

// FilterHash returns a stable, truncated SHA-256 digest for normalized filter
// values and resource identifiers. Callers should pass a struct (preferred) or
// other JSON value whose representation includes every predicate that can
// change the result set.
func FilterHash(filters any) (string, error) {
	encoded, err := json.Marshal(filters)
	if err != nil {
		return "", fmt.Errorf("marshal cursor filters: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:filterHashBytes]), nil
}

// MustFilterHash is intended for fixed, JSON-marshalable filter structs.
func MustFilterHash(filters any) string {
	hash, err := FilterHash(filters)
	if err != nil {
		panic(err)
	}
	return hash
}

// Encode serializes values into an unpadded base64url JSON cursor.
func Encode(kind, filterHash string, values any) (string, error) {
	if kind == "" || filterHash == "" {
		return "", fmt.Errorf("%w: missing kind or filter hash", ErrInvalidCursor)
	}
	encodedValues, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal cursor values: %w", err)
	}
	envelope := CursorEnvelope{
		Version:    CursorVersion,
		Kind:       kind,
		FilterHash: filterHash,
		Values:     encodedValues,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("marshal cursor envelope: %w", err)
	}
	cursor := base64.RawURLEncoding.EncodeToString(encoded)
	if len(cursor) > MaxEncodedCursorLen {
		return "", ErrCursorTooLarge
	}
	return cursor, nil
}

// Decode validates and decodes cursor into values. It rejects unknown fields
// so malformed or accidentally incompatible cursor shapes fail closed.
func Decode(cursor, kind, filterHash string, values any) error {
	if cursor == "" {
		return fmt.Errorf("%w: empty cursor", ErrInvalidCursor)
	}
	if len(cursor) > MaxEncodedCursorLen {
		return ErrCursorTooLarge
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("%w: invalid base64url encoding", ErrInvalidCursor)
	}
	if len(decoded) == 0 || len(decoded) > MaxEncodedCursorLen {
		return ErrCursorTooLarge
	}

	var envelope CursorEnvelope
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("%w: invalid envelope", ErrInvalidCursor)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return fmt.Errorf("%w: trailing data", ErrInvalidCursor)
	}
	if envelope.Version != CursorVersion {
		return ErrUnsupportedVersion
	}
	if envelope.Kind != kind {
		return ErrCursorKindMismatch
	}
	if envelope.FilterHash != filterHash {
		return ErrCursorFilterChanged
	}
	if len(envelope.Values) == 0 || bytes.Equal(envelope.Values, []byte("null")) {
		return fmt.Errorf("%w: missing values", ErrInvalidCursor)
	}

	valuesDecoder := json.NewDecoder(bytes.NewReader(envelope.Values))
	valuesDecoder.DisallowUnknownFields()
	if err := valuesDecoder.Decode(values); err != nil {
		return fmt.Errorf("%w: invalid values", ErrInvalidCursor)
	}
	if err := requireJSONEOF(valuesDecoder); err != nil {
		return fmt.Errorf("%w: trailing values", ErrInvalidCursor)
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrInvalidCursor
		}
		return err
	}
	return nil
}

// TrimPage removes the limit+1 sentinel item and reports whether another page
// exists. It deliberately does not construct a cursor because each endpoint's
// cursor tuple is different.
func TrimPage[T any](rows []T, limit int) ([]T, bool) {
	if limit < 0 {
		limit = 0
	}
	if len(rows) <= limit {
		return rows, false
	}
	return rows[:limit], true
}
