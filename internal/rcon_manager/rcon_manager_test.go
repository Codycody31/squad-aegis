package rcon_manager

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateCommandResponseRejectsMismatchedPayloads(t *testing.T) {
	tests := []struct {
		command  string
		response string
	}{
		{
			command:  "ShowServerInfo",
			response: "----- Active Players -----\n----- Recently Disconnected Players [Max of 15] -----",
		},
		{
			command:  "ListPlayers",
			response: `{"ServerName_s":"Test"}`,
		},
		{
			command:  "ListSquads",
			response: "Current level is Gorodok, layer is AAS v1, factions USA RGF",
		},
	}

	for _, tt := range tests {
		if err := validateCommandResponse(tt.command, tt.response); err == nil {
			t.Fatalf("expected validation error for %s", tt.command)
		}
	}
}

func TestValidateCommandResponseAcceptsExpectedPayloads(t *testing.T) {
	tests := []struct {
		command  string
		response string
	}{
		{
			command:  "ShowServerInfo",
			response: `{"ServerName_s":"Test","TeamOne_s":"Blue","TeamTwo_s":"Red"}`,
		},
		{
			command:  "ListPlayers",
			response: "----- Active Players -----\n----- Recently Disconnected Players [Max of 15] -----",
		},
		{
			command:  "ListSquads",
			response: "----- Active Squads -----\nTeam ID: 1 (Blue)\nTeam ID: 2 (Red)",
		},
		{
			command:  "ShowNextMap",
			response: "Next map is not defined",
		},
		{
			command:  "ShowNextMap",
			response: "Next map is Gorodok, layer is AAS v1, factions USA RGF",
		},
	}

	for _, tt := range tests {
		if err := validateCommandResponse(tt.command, tt.response); err != nil {
			t.Fatalf("expected %s payload to validate, got error: %v", tt.command, err)
		}
	}
}

func TestDefaultRetriesForCommand(t *testing.T) {
	if retries := defaultRetriesForCommand("ShowServerInfo"); retries != 2 {
		t.Fatalf("expected ShowServerInfo retries to be 2, got %d", retries)
	}

	if retries := defaultRetriesForCommand("ListPlayers"); retries != 2 {
		t.Fatalf("expected ListPlayers retries to be 2, got %d", retries)
	}

	if retries := defaultRetriesForCommand("AdminKick 1 testing"); retries != 1 {
		t.Fatalf("expected AdminKick retries to remain 1, got %d", retries)
	}
}

func TestExecuteCommandWithOptionsCreatesFreshAttemptContext(t *testing.T) {
	managerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverID := uuid.New()
	conn := &ServerConnection{
		CommandChan: make(chan RconCommand, 2),
	}

	manager := &RconManager{
		connections: map[uuid.UUID]*ServerConnection{
			serverID: conn,
		},
		ctx: managerCtx,
	}

	received := make(chan RconCommand, 2)
	go func() {
		for i := 0; i < 2; i++ {
			received <- <-conn.CommandChan
		}
	}()

	_, err := manager.ExecuteCommandWithOptions(serverID, "ListPlayers", CommandOptions{
		Timeout:  20 * time.Millisecond,
		Retries:  2,
		Context:  context.Background(),
		Priority: PriorityNormal,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "command execution timeout") {
		t.Fatalf("expected execution timeout, got %v", err)
	}

	timeout := time.After(200 * time.Millisecond)
	for i := 0; i < 2; i++ {
		select {
		case <-received:
		case <-timeout:
			t.Fatalf("expected 2 command attempts to reach the queue, got %d", i)
		}
	}
}

func TestExecuteCommandWithOptionsDoesNotCloseLateResponseChannel(t *testing.T) {
	managerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverID := uuid.New()
	conn := &ServerConnection{
		CommandChan: make(chan RconCommand, 1),
	}

	manager := &RconManager{
		connections: map[uuid.UUID]*ServerConnection{
			serverID: conn,
		},
		ctx: managerCtx,
	}

	captured := make(chan RconCommand, 1)
	go func() {
		captured <- <-conn.CommandChan
	}()

	_, err := manager.ExecuteCommandWithOptions(serverID, "ListSquads", CommandOptions{
		Timeout:  20 * time.Millisecond,
		Retries:  1,
		Context:  context.Background(),
		Priority: PriorityNormal,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}

	cmd := <-captured

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("late response send panicked: %v", r)
		}
	}()

	cmd.Response <- CommandResponse{Response: "late", Error: nil}
}

func TestExecuteCommandWithOptionsDoesNotRetryNonRetryableErrors(t *testing.T) {
	managerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverID := uuid.New()
	conn := &ServerConnection{
		CommandChan: make(chan RconCommand, 2),
	}

	manager := &RconManager{
		connections: map[uuid.UUID]*ServerConnection{
			serverID: conn,
		},
		ctx: managerCtx,
	}

	go func() {
		cmd := <-conn.CommandChan
		cmd.Response <- CommandResponse{Error: context.Canceled}
	}()

	_, err := manager.ExecuteCommandWithOptions(serverID, "ListPlayers", CommandOptions{
		Timeout:  50 * time.Millisecond,
		Retries:  2,
		Context:  context.Background(),
		Priority: PriorityNormal,
	})
	if err == nil {
		t.Fatal("expected non-retryable error")
	}

	select {
	case <-conn.CommandChan:
		t.Fatal("expected non-retryable error to stop retries")
	case <-time.After(100 * time.Millisecond):
	}
}
