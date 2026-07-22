package watcher

import "sync"

// Address identifies a Sauerbraten server. The type is retained for protocol
// compatibility, but this Shring fork intentionally does not query the public
// Sauerbraten master server.
type Address struct {
	Host string
	Port int
}

// Server contains the compact server-info payload understood by the browser
// client. Public servers are never populated in this fork.
type Server struct {
	Info   []byte `cbor:"info"`
	Length int
}

type Servers map[Address]Server

// Watcher is kept as a compatibility layer for the existing WebSocket ingress.
// It performs no external discovery, DNS lookups, UDP probes, or master-server
// polling. The browser list is populated only from servers in this Sour cluster.
type Watcher struct {
	serverMutex sync.RWMutex
	servers     Servers
}

func NewWatcher() *Watcher {
	return &Watcher{servers: make(Servers)}
}

// FetchServers deliberately returns no public servers.
func FetchServers() ([]Address, error) {
	return []Address{}, nil
}

func (watcher *Watcher) UpdateServerList() {}
func (watcher *Watcher) PingServers()       {}
func (watcher *Watcher) ReceivePings()      {}

// Get returns a defensive copy so callers cannot race with future local server
// registration code.
func (watcher *Watcher) Get() Servers {
	watcher.serverMutex.RLock()
	defer watcher.serverMutex.RUnlock()

	servers := make(Servers, len(watcher.servers))
	for address, server := range watcher.servers {
		copyServer := server
		copyServer.Info = append([]byte(nil), server.Info...)
		servers[address] = copyServer
	}
	return servers
}

// Watch intentionally does nothing. Calling it remains safe for older startup
// paths while guaranteeing that no third-party servers can enter the list.
func (watcher *Watcher) Watch() error {
	return nil
}
