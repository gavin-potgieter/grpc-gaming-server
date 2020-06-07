package channel

import (
	"fmt"
	"sync"
)

// Channel is a channel of interface{}
type Channel struct {
	Events       chan interface{}
	Recovered    chan bool
	SkipBack     chan interface{}
	lock         sync.RWMutex
	open         bool
	reconnecting bool
	listening    bool
}

// NewChannel creates a new channel
func NewChannel() *Channel {
	return &Channel{
		Events:       make(chan interface{}, 1000),
		Recovered:    make(chan bool, 1),
		SkipBack:     make(chan interface{}, 1),
		lock:         sync.RWMutex{},
		open:         true,
		listening:    false,
		reconnecting: false,
	}
}

// Send an event
func (channel *Channel) Send(event interface{}) {
	channel.lock.RLock()
	defer channel.lock.RUnlock()
	if !channel.open {
		return
	}
	channel.Events <- event
}

// Retry an event
func (channel *Channel) Retry(event interface{}) {
	channel.lock.RLock()
	defer channel.lock.RUnlock()
	if !channel.open {
		return
	}
	channel.reconnecting = true
	if event != nil {
		channel.SkipBack <- event
	}
}

// Close a channel
func (channel *Channel) Close() {
	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.open {
		close(channel.Events)
		close(channel.Recovered)
		close(channel.SkipBack)
		channel.open = false
	}
}

// Recover signals recovery
func (channel *Channel) Recover() {
	channel.lock.RLock()
	defer channel.lock.RUnlock()
	if channel.open && channel.reconnecting {
		channel.Recovered <- true
		channel.reconnecting = false
	}
}

// Listen enables listening
func (channel *Channel) Listen() error {
	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.listening {
		return fmt.Errorf("client_already_listening")
	}
	channel.listening = true
	return nil
}

// Hangup disables listening
func (channel *Channel) Hangup() {
	channel.lock.Lock()
	defer channel.lock.Unlock()
	channel.listening = false
}
