package main

import (
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
}

// NewChannel creates a new channel
func NewChannel() *Channel {
	return &Channel{
		Events:    make(chan interface{}, 100),
		Recovered: make(chan bool, 1),
		SkipBack:  make(chan interface{}, 1),
		lock:      sync.RWMutex{},
		open:      true,
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
	channel.SkipBack <- event
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
	if channel.open {
		channel.Recovered <- true
	}
}
