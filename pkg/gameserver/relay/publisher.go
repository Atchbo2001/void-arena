package relay

import (
	"sync"

	"github.com/cfoust/sour/pkg/game/protocol"
)

const publisherQueueSize = 128

// Publisher provides methods to send updates to all subscribers of a certain topic.
// It is safe to close while another goroutine is publishing.
type Publisher struct {
	cn          uint32
	notifyRelay chan<- uint32
	updates     chan []protocol.Message
	done        chan struct{}
	closeOnce   sync.Once
}

func newPublisher(cn uint32, notifyRelay chan<- uint32) (*Publisher, <-chan []protocol.Message) {
	updates := make(chan []protocol.Message, publisherQueueSize)

	p := &Publisher{
		cn:          cn,
		notifyRelay: notifyRelay,
		updates:     updates,
		done:        make(chan struct{}),
	}

	return p, updates
}

// Publish queues an update before notifying the relay. This ordering prevents a
// disconnect race where the relay observes a notification after the source has
// already been removed and leaves the publisher blocked forever.
func (p *Publisher) Publish(messages ...protocol.Message) {
	if len(messages) == 0 {
		return
	}

	copied := append([]protocol.Message(nil), messages...)
	select {
	case <-p.done:
		return
	case p.updates <- copied:
	}

	select {
	case <-p.done:
		return
	case p.notifyRelay <- p.cn:
	}
}

// Close makes future publishes return immediately. The update channel is not
// closed because a concurrent sender could otherwise panic; the relay drops the
// channel when the source is removed.
func (p *Publisher) Close() {
	p.closeOnce.Do(func() { close(p.done) })
}
