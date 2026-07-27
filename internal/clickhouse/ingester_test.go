package clickhouse

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
)

func TestSupportedEventTypes(t *testing.T) {
	supported := []event_manager.EventType{
		event_manager.EventTypeRconChatMessage,
		event_manager.EventTypeRconServerInfo,
		event_manager.EventTypeLogPlayerConnected,
		event_manager.EventTypeLogPlayerDisconnected,
		event_manager.EventTypeLogPlayerDamaged,
		event_manager.EventTypeLogPlayerDied,
		event_manager.EventTypeLogPlayerWounded,
		event_manager.EventTypeLogPlayerRevived,
		event_manager.EventTypeLogPlayerPossess,
		event_manager.EventTypeLogJoinSucceeded,
		event_manager.EventTypeLogAdminBroadcast,
		event_manager.EventTypeLogDeployableDamaged,
		event_manager.EventTypeLogTickRate,
		event_manager.EventTypeLogGameEventUnified,
	}

	for _, eventType := range supported {
		if !isSupportedEventType(eventType) {
			t.Errorf("isSupportedEventType(%q) = false", eventType)
		}
	}
	if isSupportedEventType(event_manager.EventTypePluginCustom) {
		t.Fatal("plugin custom events must not enter the ClickHouse game-event ingester")
	}
}

func TestNewEventIngesterUsesProductionBatchDefaults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ingester := NewEventIngester(ctx, nil, nil)
	defer ingester.writeCancel()

	if ingester.batchSize != 1000 {
		t.Fatalf("batchSize = %d, want 1000", ingester.batchSize)
	}
	if cap(ingester.eventQueue) != 10000 {
		t.Fatalf("eventQueue capacity = %d, want 10000", cap(ingester.eventQueue))
	}
	if ingester.flushInterval != 5*time.Second {
		t.Fatalf("flushInterval = %s, want 5s", ingester.flushInterval)
	}
}

func TestProcessEventUsesBackpressureAndRejectsUnsupportedTypes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writeCtx, writeCancel := context.WithCancel(context.Background())
	defer writeCancel()

	ingester := &EventIngester{
		ctx:           ctx,
		writeCtx:      writeCtx,
		eventQueue:    make(chan *IngestEvent, 1),
		batchSize:     1000,
		retryBackoffs: [3]time.Duration{},
	}

	ingester.processEvent(event_manager.Event{Type: event_manager.EventTypePluginCustom})
	if got := ingester.Stats().InvalidRows; got != 1 {
		t.Fatalf("invalid rows = %d, want 1", got)
	}

	first := event_manager.Event{
		ID:        uuid.New(),
		Type:      event_manager.EventTypeRconChatMessage,
		Timestamp: time.Now(),
	}
	second := first
	second.ID = uuid.New()
	ingester.processEvent(first)

	enqueued := make(chan struct{})
	go func() {
		ingester.processEvent(second)
		close(enqueued)
	}()

	select {
	case <-enqueued:
		t.Fatal("second event bypassed backpressure while the queue was full")
	case <-time.After(20 * time.Millisecond):
	}

	<-ingester.eventQueue
	select {
	case <-enqueued:
	case <-time.After(time.Second):
		t.Fatal("second event did not enqueue after capacity became available")
	}
}

func TestBatchProcessorMaintainsIndependentTypeBuffersAndFlushesOnClose(t *testing.T) {
	writeCtx, writeCancel := context.WithCancel(context.Background())
	defer writeCancel()

	ingester := &EventIngester{
		writeCtx:      writeCtx,
		batchSize:     2,
		flushInterval: time.Hour,
		eventQueue:    make(chan *IngestEvent, 4),
	}

	var mu sync.Mutex
	flushed := make(map[event_manager.EventType][]int)
	ingester.ingestOverride = func(
		_ context.Context,
		eventType event_manager.EventType,
		events []*IngestEvent,
	) error {
		mu.Lock()
		defer mu.Unlock()
		flushed[eventType] = append(flushed[eventType], len(events))
		return nil
	}

	done := make(chan struct{})
	go func() {
		ingester.batchProcessor()
		close(done)
	}()

	ingester.eventQueue <- &IngestEvent{EventType: event_manager.EventTypeRconChatMessage}
	ingester.eventQueue <- &IngestEvent{EventType: event_manager.EventTypeLogTickRate}
	ingester.eventQueue <- &IngestEvent{EventType: event_manager.EventTypeLogTickRate}
	close(ingester.eventQueue)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("batch processor did not drain and flush a closed queue")
	}

	mu.Lock()
	defer mu.Unlock()
	if got := flushed[event_manager.EventTypeRconChatMessage]; len(got) != 1 || got[0] != 1 {
		t.Fatalf("chat flushes = %v, want [1]", got)
	}
	if got := flushed[event_manager.EventTypeLogTickRate]; len(got) != 1 || got[0] != 2 {
		t.Fatalf("tick-rate flushes = %v, want [2]", got)
	}
	if got := ingester.Stats().BatchRows; got != 3 {
		t.Fatalf("batch rows = %d, want 3", got)
	}
}

func TestFlushEventTypeRetriesAndMarksDegraded(t *testing.T) {
	writeCtx, writeCancel := context.WithCancel(context.Background())
	defer writeCancel()

	ingester := &EventIngester{
		writeCtx:      writeCtx,
		retryBackoffs: [3]time.Duration{},
	}

	calls := 0
	ingester.ingestOverride = func(
		context.Context,
		event_manager.EventType,
		[]*IngestEvent,
	) error {
		calls++
		return errors.New("insert failed")
	}

	events := []*IngestEvent{{}, {}, {}}
	ingester.flushEventType(event_manager.EventTypeLogTickRate, events)

	stats := ingester.Stats()
	if calls != 4 {
		t.Fatalf("insert attempts = %d, want 4", calls)
	}
	if stats.RetryCount != 3 {
		t.Fatalf("retry count = %d, want 3", stats.RetryCount)
	}
	if stats.FlushFailures != 1 || stats.FailedRows != 3 || !stats.Degraded {
		t.Fatalf("unexpected failure stats: %+v", stats)
	}
}

func TestInvalidPayloadIsNotAppended(t *testing.T) {
	ingester := &EventIngester{}
	err := ingester.ingestChatMessages(context.Background(), []*IngestEvent{{
		Data: &event_manager.RconServerInfoData{},
	}})
	if err != nil {
		t.Fatalf("ingestChatMessages() error = %v", err)
	}
	if got := ingester.Stats().InvalidRows; got != 1 {
		t.Fatalf("invalid rows = %d, want 1", got)
	}
}
