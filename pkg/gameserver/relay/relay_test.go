package relay

import (
	"testing"
	"time"

	"github.com/cfoust/sour/pkg/game/protocol"
)

type delivered struct {
	channel uint8
	payload []protocol.Message
}

func payloadHasText(payload []protocol.Message, expected string) bool {
	for _, message := range payload {
		text, ok := message.(protocol.Text)
		if ok && text.Text == expected {
			return true
		}
	}
	return false
}

func TestSingleClientPacketsAreNotReplayedToLateJoiner(t *testing.T) {
	r := New()
	first, _ := r.AddClient(1, func(uint8, []protocol.Message) {})

	first.Publish(protocol.Text{Text: "stale"})
	time.Sleep(40 * time.Millisecond)

	received := make(chan delivered, 4)
	_, _ = r.AddClient(2, func(channel uint8, payload []protocol.Message) {
		received <- delivered{channel: channel, payload: payload}
	})

	first.Publish(protocol.Text{Text: "fresh"})

	select {
	case packet := <-received:
		if packet.channel != 1 {
			t.Fatalf("expected channel 1, got %d", packet.channel)
		}
		if payloadHasText(packet.payload, "stale") {
			t.Fatal("late joiner received stale packet")
		}
		if !payloadHasText(packet.payload, "fresh") {
			t.Fatal("fresh packet was not delivered")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fresh packet")
	}
}

func TestPublisherCloseDuringRemoveDoesNotDeadlock(t *testing.T) {
	r := New()
	positions, packets := r.AddClient(7, func(uint8, []protocol.Message) {})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 64; i++ {
			positions.Publish(protocol.Text{Text: "position"})
			packets.Publish(protocol.Text{Text: "packet"})
		}
	}()

	_ = r.RemoveClient(7)
	positions.Close()
	packets.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher remained blocked after client removal")
	}
}

func TestHumanPacketsReachClientZero(t *testing.T) {
	r := New()
	firstMessages := make(chan delivered, 8)
	_, _ = r.AddClient(0, func(channel uint8, payload []protocol.Message) {
		firstMessages <- delivered{channel: channel, payload: payload}
	})
	secondPositions, secondPackets := r.AddClient(1, func(uint8, []protocol.Message) {})

	secondPositions.Publish(protocol.Text{Text: "movement"})
	secondPackets.Publish(protocol.Text{Text: "spawn"})

	seenMovement := false
	seenReliable := false
	timeout := time.After(time.Second)
	for !seenMovement || !seenReliable {
		select {
		case packet := <-firstMessages:
			switch packet.channel {
			case 0:
				seenMovement = payloadHasText(packet.payload, "movement") || seenMovement
			case 1:
				seenReliable = payloadHasText(packet.payload, "spawn") || seenReliable
			}
		case <-timeout:
			t.Fatalf("client zero missed relayed traffic: movement=%t reliable=%t", seenMovement, seenReliable)
		}
	}
}

func TestThreeHumanClientsReceiveEveryOtherReliablePacket(t *testing.T) {
	r := New()
	received := []chan delivered{
		make(chan delivered, 8),
		make(chan delivered, 8),
		make(chan delivered, 8),
	}

	publishers := make([]*Publisher, 3)
	for cn := uint32(0); cn < 3; cn++ {
		index := int(cn)
		_, publishers[index] = r.AddClient(cn, func(channel uint8, payload []protocol.Message) {
			received[index] <- delivered{channel: channel, payload: payload}
		})
	}

	for cn, publisher := range publishers {
		publisher.Publish(protocol.Text{Text: "from-" + string(rune('0'+cn))})
	}

	for receiver := 0; receiver < 3; receiver++ {
		expected := map[string]bool{}
		for sender := 0; sender < 3; sender++ {
			if sender != receiver {
				expected["from-"+string(rune('0'+sender))] = false
			}
		}

		timeout := time.After(time.Second)
		for {
			complete := true
			for _, seen := range expected {
				complete = complete && seen
			}
			if complete {
				break
			}

			select {
			case packet := <-received[receiver]:
				if packet.channel != 1 {
					continue
				}
				own := "from-" + string(rune('0'+receiver))
				if payloadHasText(packet.payload, own) {
					t.Fatalf("client %d received its own reliable packet", receiver)
				}
				for text := range expected {
					if payloadHasText(packet.payload, text) {
						expected[text] = true
					}
				}
			case <-timeout:
				t.Fatalf("client %d missed reliable packets: %#v", receiver, expected)
			}
		}
	}
}

func TestBotSourceDoesNotEchoToOwner(t *testing.T) {
	r := New()
	ownerMessages := make(chan delivered, 2)
	otherMessages := make(chan delivered, 2)
	_, _ = r.AddClient(1, func(channel uint8, payload []protocol.Message) {
		ownerMessages <- delivered{channel: channel, payload: payload}
	})
	_, _ = r.AddClient(2, func(channel uint8, payload []protocol.Message) {
		otherMessages <- delivered{channel: channel, payload: payload}
	})
	_, botPackets := r.AddSource(128, 1)

	botPackets.Publish(protocol.Text{Text: "bot"})

	select {
	case <-ownerMessages:
		t.Fatal("bot packet echoed back to its simulation owner")
	case <-time.After(40 * time.Millisecond):
	}

	select {
	case <-otherMessages:
	case <-time.After(time.Second):
		t.Fatal("bot packet was not delivered to another player")
	}
}
