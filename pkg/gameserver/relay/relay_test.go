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
		foundFresh := false
		for _, message := range packet.payload {
			text, ok := message.(protocol.Text)
			if !ok {
				continue
			}
			if text.Text == "stale" {
				t.Fatal("late joiner received stale packet")
			}
			if text.Text == "fresh" {
				foundFresh = true
			}
		}
		if !foundFresh {
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
