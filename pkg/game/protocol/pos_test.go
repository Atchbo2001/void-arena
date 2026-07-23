package protocol

import (
	"testing"

	gameio "github.com/cfoust/sour/pkg/game/io"
)

func TestBotPositionUsesUnsignedClientNumber(t *testing.T) {
	state := PhysicsState{
		State:        4,
		LifeSequence: 1,
		Yaw:          90,
		Pitch:        0,
		Roll:         0,
		O:            Vec{X: 10, Y: 20, Z: 30},
	}

	encoded, err := Encode(
		Pos{Client: 128, State: state},
		Ping{Cmillis: 1234},
	)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	messages, err := Decode(encoded, true)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	pos, ok := messages[0].(Pos)
	if !ok {
		t.Fatalf("first message is %T, expected Pos", messages[0])
	}
	if pos.Client != 128 {
		t.Fatalf("client number = %d, expected 128", pos.Client)
	}
	if _, ok := messages[1].(Ping); !ok {
		t.Fatalf("second message is %T, expected Ping", messages[1])
	}

	// Confirm the wire encoding really is putuint rather than putint.
	p := gameio.Packet(encoded)
	if code, ok := p.GetInt(); !ok || MessageCode(code) != N_POS {
		t.Fatal("missing N_POS prefix")
	}
	if cn, ok := p.GetUint(); !ok || cn != 128 {
		t.Fatalf("wire client number = %d, expected 128", cn)
	}
}
