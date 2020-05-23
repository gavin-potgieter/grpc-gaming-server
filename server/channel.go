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
		Channel: nil,
		lock:    sync.RWMutex{},
		open:    false,
	}
}

// Open a channel
func (channel *Channel) Open() {
	channel.lock.Lock()
	defer channel.lock.Unlock()
	if channel.open {
		return
	}
	channel.Channel = make(chan interface{}, 100)
	channel.open = true
	return
}

// Send an event
func (channel *Channel) Send(event interface{}) {
	channel.lock.RLock()
	defer channel.lock.RLock()
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
