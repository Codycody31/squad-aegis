package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

var mockBuf = []byte{14, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 84, 101, 115, 116, 0, 0}

func Test_encode(t *testing.T) {
	result := Encode(1, 1, "Test")

	if diff := cmp.Diff(mockBuf, result); diff != "" {
		t.Error(diff)
	}
}

func Test_decode(t *testing.T) {
	expected := RconResponse{
		Size: 14,
		ID:   1,
		Type: 1,
		Body: "Test",
	}

	result := Decode(mockBuf)

	if diff := cmp.Diff(expected, result); diff != "" {
		t.Error(diff)
	}
}
