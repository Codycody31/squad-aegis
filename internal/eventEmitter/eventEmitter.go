// Source: https://github.com/iamalone98/eventEmitter

package eventEmitter

import (
	"reflect"
	"sync"
)

type ListenerCallback func(interface{})
type Listener struct {
	once     bool
	callback ListenerCallback
}

type EventEmitter interface {
	On(event string, callback ListenerCallback)
	Once(event string, callback ListenerCallback)
	Emit(event string, data interface{})
	RemoveListener(event string, callback ListenerCallback)
	RemoveAllListeners()
	RemoveAllListenersByEvent(event string)
}

type eventEmitter struct {
	listeners map[string][]Listener
	mu        sync.Mutex
}

func NewEventEmitter() EventEmitter {
	return &eventEmitter{
		listeners: make(map[string][]Listener),
	}
}

func (e *eventEmitter) On(event string, callback ListenerCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners[event] = append(e.listeners[event], Listener{callback: callback, once: false})
}

func (e *eventEmitter) Once(event string, callback ListenerCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners[event] = append(e.listeners[event], Listener{callback: callback, once: true})
}

func (e *eventEmitter) Emit(event string, data interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Emit for listeners registered for the specific event.
	if listeners, ok := e.listeners[event]; ok {
		// Iterate over a copy of the slice to avoid issues with removal.
		for i, v := range listeners {
			go func(i int, v Listener) {
				v.callback(data)
				if v.once {
					e.mu.Lock()
					defer e.mu.Unlock()
					// Remove the once listener.
					e.listeners[event] = append(listeners[:i], listeners[i+1:]...)
				}
			}(i, v)
		}
	}

	// Emit for wildcard listeners registered under "*".
	if wildcardListeners, ok := e.listeners["*"]; ok {
		// For wildcard callbacks, pass a tuple with the event name and data.
		wildcardData := []interface{}{event, data}
		for i, v := range wildcardListeners {
			go func(i int, v Listener) {
				v.callback(wildcardData)
				if v.once {
					e.mu.Lock()
					defer e.mu.Unlock()
					e.listeners["*"] = append(wildcardListeners[:i], wildcardListeners[i+1:]...)
				}
			}(i, v)
		}
	}
}

func (e *eventEmitter) RemoveListener(event string, callback ListenerCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()

	listenerPtr := reflect.ValueOf(callback).Pointer()

	if listeners, ok := e.listeners[event]; ok {
		for i, v := range listeners {
			ptr := reflect.ValueOf(v.callback).Pointer()
			if ptr == listenerPtr {
				e.listeners[event] = append(listeners[:i], listeners[i+1:]...)
				break
			}
		}
	}
}

func (e *eventEmitter) RemoveAllListeners() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = make(map[string][]Listener)
}

func (e *eventEmitter) RemoveAllListenersByEvent(event string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.listeners, event)
}
