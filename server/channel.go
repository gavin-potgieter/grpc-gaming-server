package main

import (
	"sync"
)

// Channel is a channel of interface{}
type Channel struct {
	Channel   chan interface{}
	lock      sync.RWMutex
	open      bool
	Recovered chan bool
}

// NewChannel creates a new channel
func NewChannel() *Channel {
	return &Channel{
		Channel:   make(chan interface{}, 100),
		lock:      sync.RWMutex{},
		open:      true,
		Recovered: make(chan bool, 5),
	}
}

// Send an event
func (channel *Channel) Send(event interface{}) {
	channel.lock.RLock()
	defer channel.lock.RUnlock()
	if !channel.open {
		return
	}
	channel.Channel <- event
}

// Close a channel
func (channel *Channel) Close() {
	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.open {
		close(channel.Channel)
	}
}

// Recover signals recovery
func (channel *Channel) Recover() {
	channel.Recovered <- true
}
