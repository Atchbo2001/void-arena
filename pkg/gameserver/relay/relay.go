package relay

import (
	"sort"
	"time"

	"github.com/cfoust/sour/pkg/game/protocol"
	"github.com/sasha-s/go-deadlock"
)

type sendFunc func(channel uint8, payload []protocol.Message)

type queuedSend struct {
	fn      sendFunc
	channel uint8
	payload []protocol.Message
}

// Relay relays positional and client packet data between clients.
type Relay struct {
	mutex deadlock.Mutex

	incPositionsNotifs chan uint32
	incPositions       map[uint32]<-chan []protocol.Message
	positions          map[uint32][]protocol.Message

	incClientPacketsNotifs chan uint32
	incClientPackets       map[uint32]<-chan []protocol.Message
	clientPackets          map[uint32][]protocol.Message

	// send contains only real network recipients. Bot/AI sources may publish but
	// are not added here because their owner simulates them locally.
	send        map[uint32]sendFunc
	sourceOwner map[uint32]uint32
}

func New() *Relay {
	r := &Relay{
		incPositionsNotifs: make(chan uint32, 512),
		incPositions:       map[uint32]<-chan []protocol.Message{},
		positions:          map[uint32][]protocol.Message{},

		incClientPacketsNotifs: make(chan uint32, 512),
		incClientPackets:       map[uint32]<-chan []protocol.Message{},
		clientPackets:          map[uint32][]protocol.Message{},

		send:        map[uint32]sendFunc{},
		sourceOwner: map[uint32]uint32{},
	}

	go r.loop()
	return r
}

func (r *Relay) loop() {
	ticker := time.NewTicker(11 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.flush(r.positions, func(uint32, []protocol.Message) []protocol.Message { return nil }, 0)
			r.flush(r.clientPackets, func(cn uint32, pkt []protocol.Message) []protocol.Message {
				return []protocol.Message{protocol.ClientPacket{Client: int32(cn)}}
			}, 1)
		case cn := <-r.incPositionsNotifs:
			r.receive(cn, r.incPositions, func(pos []protocol.Message) {
				if len(pos) == 0 {
					delete(r.positions, cn)
					return
				}
				r.positions[cn] = pos
			})
		case cn := <-r.incClientPacketsNotifs:
			r.receive(cn, r.incClientPackets, func(pkt []protocol.Message) {
				r.clientPackets[cn] = append(r.clientPackets[cn], pkt...)
			})
		}
	}
}

func (r *Relay) addSourceLocked(cn uint32) (positions *Publisher, packets *Publisher) {
	delete(r.incPositions, cn)
	delete(r.positions, cn)
	delete(r.incClientPackets, cn)
	delete(r.clientPackets, cn)

	positions, posCh := newPublisher(cn, r.incPositionsNotifs)
	r.incPositions[cn] = posCh
	packets, pktCh := newPublisher(cn, r.incClientPacketsNotifs)
	r.incClientPackets[cn] = pktCh
	return
}

func (r *Relay) AddClient(cn uint32, sf sendFunc) (positions *Publisher, packets *Publisher) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.forceRemoveClient(cn)
	r.send[cn] = sf
	return r.addSourceLocked(cn)
}

// AddSource registers a source that can publish state but does not directly
// receive network packets. This is used by AI clients simulated by a human owner.
func (r *Relay) AddSource(cn uint32, ownerCN uint32) (positions *Publisher, packets *Publisher) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.forceRemoveClient(cn)
	r.sourceOwner[cn] = ownerCN
	return r.addSourceLocked(cn)
}

func (r *Relay) SetSourceOwner(cn uint32, ownerCN uint32) {
	r.mutex.Lock()
	if _, exists := r.incPositions[cn]; exists {
		r.sourceOwner[cn] = ownerCN
	}
	r.mutex.Unlock()
}

func (r *Relay) RemoveClient(cn uint32) error {
	r.mutex.Lock()
	r.forceRemoveClient(cn)
	r.mutex.Unlock()
	return nil
}

func (r *Relay) forceRemoveClient(cn uint32) {
	delete(r.incPositions, cn)
	delete(r.positions, cn)
	delete(r.incClientPackets, cn)
	delete(r.clientPackets, cn)
	delete(r.send, cn)
	delete(r.sourceOwner, cn)
}

func (r *Relay) recipientSnapshot(excludeCN uint32) []sendFunc {
	order := make([]uint32, 0, len(r.send))
	ownerCN, hasOwner := r.sourceOwner[excludeCN]
	for cn := range r.send {
		if cn != excludeCN && (!hasOwner || cn != ownerCN) {
			order = append(order, cn)
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	result := make([]sendFunc, 0, len(order))
	for _, cn := range order {
		result = append(result, r.send[cn])
	}
	return result
}

func (r *Relay) FlushPositionAndSend(cn uint32, p protocol.Message) {
	r.mutex.Lock()
	positions := append([]protocol.Message(nil), r.positions[cn]...)
	delete(r.positions, cn)
	recipients := r.recipientSnapshot(cn)
	r.mutex.Unlock()

	for _, send := range recipients {
		if len(positions) > 0 {
			send(0, positions)
		}
		send(0, []protocol.Message{p})
	}
}

func (r *Relay) receive(cn uint32, from map[uint32]<-chan []protocol.Message, process func(upd []protocol.Message)) {
	r.mutex.Lock()
	ch, ok := from[cn]
	r.mutex.Unlock()
	if !ok {
		return
	}

	select {
	case update := <-ch:
		r.mutex.Lock()
		if current, exists := from[cn]; exists && current == ch {
			process(update)
		}
		r.mutex.Unlock()
	default:
		// Notification and update are normally adjacent, but never stall the
		// relay loop if a source disappears in between them.
	}
}

func (r *Relay) flush(packets map[uint32][]protocol.Message, prefix func(uint32, []protocol.Message) []protocol.Message, channel uint8) {
	r.mutex.Lock()
	if len(packets) == 0 {
		r.mutex.Unlock()
		return
	}

	// Always clear this tick's packets, even with one or zero recipients. Keeping
	// them caused stale movement/fire events to be replayed when another player
	// joined later, one of the primary intermittent desync symptoms.
	packetSnapshot := make(map[uint32][]protocol.Message, len(packets))
	for cn, packet := range packets {
		if len(packet) > 0 {
			packetSnapshot[cn] = append([]protocol.Message(nil), packet...)
		}
		delete(packets, cn)
	}

	receiverCNs := make([]uint32, 0, len(r.send))
	receivers := make(map[uint32]sendFunc, len(r.send))
	sourceOwners := make(map[uint32]uint32, len(r.sourceOwner))
	for source, owner := range r.sourceOwner {
		sourceOwners[source] = owner
	}
	for cn, send := range r.send {
		receiverCNs = append(receiverCNs, cn)
		receivers[cn] = send
	}
	r.mutex.Unlock()

	if len(packetSnapshot) == 0 || len(receivers) == 0 {
		return
	}

	senderCNs := make([]uint32, 0, len(packetSnapshot))
	for cn := range packetSnapshot {
		senderCNs = append(senderCNs, cn)
	}
	sort.Slice(senderCNs, func(i, j int) bool { return senderCNs[i] < senderCNs[j] })
	sort.Slice(receiverCNs, func(i, j int) bool { return receiverCNs[i] < receiverCNs[j] })

	for _, receiverCN := range receiverCNs {
		payload := make([]protocol.Message, 0, 64)
		for _, senderCN := range senderCNs {
			if senderCN == receiverCN || sourceOwners[senderCN] == receiverCN {
				continue
			}
			packet := packetSnapshot[senderCN]
			payload = append(payload, prefix(senderCN, packet)...)
			payload = append(payload, packet...)
		}
		if len(payload) > 0 {
			receivers[receiverCN](channel, payload)
		}
	}
}
