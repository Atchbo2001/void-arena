package game

import (
	"testing"
	"time"

	"github.com/cfoust/sour/pkg/game/protocol"
)

// mockServer implements the current game.Server interface so these tests
// track the real constructor and interface signatures used in production.
var _ Server = &mockServer{}

type mockServer struct{}

func (s *mockServer) GameDuration() time.Duration   { return 10 * time.Minute }
func (s *mockServer) Broadcast(...protocol.Message) {}
func (s *mockServer) Message(string)                {}
func (s *mockServer) Intermission()                 {}
func (s *mockServer) ForEachPlayer(func(*Player))   {}
func (s *mockServer) UniqueName(p *Player) string   { return "player" }
func (s *mockServer) NumberOfPlayers() int          { return 5 }

func TestEfficCTFIsTeamFlagMode(t *testing.T) {
	s := &mockServer{}

	var mode Mode = NewEfficCTF(s, true)

	teamed, ok := mode.(TeamMode)
	if !ok {
		t.Fatal("effic ctf is not a team mode")
	}
	if _, ok := mode.(FlagMode); !ok {
		t.Fatal("effic ctf is not a flag mode")
	}

	p1, p2 := NewPlayer(1), NewPlayer(2)

	teamed.Join(&p1)
	if countPlayers(teamed) != 1 {
		t.Error("after one player joined, player count is not 1")
	}

	teamed.Join(&p2)
	if countPlayers(teamed) != 2 {
		t.Error("after two players joined, player count is not 2")
	}
}

func TestCompetitiveClockConstruction(t *testing.T) {
	s := &mockServer{}
	mode := NewEfficCTF(s, true)

	clock := NewCompetitiveClock(s, mode)

	// The production server stores this on Server.Clock and asserts
	// Clock.(Competitive) when a player spawns; keep both facts pinned here.
	var _ Clock = clock
	var _ Competitive = clock

	p := NewPlayer(1)
	clock.Spawned(&p) // must not panic before the clock is started
}

func countPlayers(tm TeamMode) (sum int) {
	tm.ForEachTeam(func(t *Team) { sum += len(t.Players) })
	return
}
