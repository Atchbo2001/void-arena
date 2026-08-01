package gameserver

import (
	"testing"
	"time"

	P "github.com/cfoust/sour/pkg/game/protocol"
	servergame "github.com/cfoust/sour/pkg/gameserver/game"
	"github.com/cfoust/sour/pkg/gameserver/protocol/gamemode"
	"github.com/cfoust/sour/pkg/gameserver/protocol/mastermode"
	"github.com/cfoust/sour/pkg/gameserver/protocol/playerstate"
	"github.com/cfoust/sour/pkg/utils"
)

type syncTestMode struct{}

func (syncTestMode) ID() gamemode.ID                         { return gamemode.FFA }
func (syncTestMode) NeedsMapInfo() bool                      { return false }
func (syncTestMode) Leave(*servergame.Player)                {}
func (syncTestMode) CanSpawn(*servergame.Player) bool        { return true }
func (syncTestMode) Spawn(*servergame.PlayerState)           {}
func (syncTestMode) HandleFrag(*servergame.Player, *servergame.Player) {}
func (syncTestMode) Pause()                                  {}
func (syncTestMode) Resume()                                 {}
func (syncTestMode) CleanUp()                                {}

type syncTestClock struct{}

func (syncTestClock) Start()                                      {}
func (syncTestClock) Pause(*servergame.Player)                    {}
func (syncTestClock) Paused() bool                                { return false }
func (syncTestClock) Resume(*servergame.Player)                   {}
func (syncTestClock) Stop()                                       {}
func (syncTestClock) Ended() bool                                 { return false }
func (syncTestClock) TimeLeft() time.Duration                     { return 10 * time.Minute }
func (syncTestClock) SetTimeLeft(time.Duration)                   {}
func (syncTestClock) Leave(*servergame.Player)                    {}
func (syncTestClock) CleanUp()                                    {}

func newSyncTestClient(cn uint32, session uint32, out chan ServerPacket) *Client {
	client := NewClient(cn, session, out)
	client.Name = "player"
	client.Model = 1
	client.Joined = true
	client.State = playerstate.Alive
	client.LifeSequence = int32(cn + 1)
	return client
}

func TestWelcomeInitializesPlayersBeforeResume(t *testing.T) {
	joinOut := make(chan ServerPacket, 4)
	existingOut := make(chan ServerPacket, 4)

	existing := newSyncTestClient(0, 100, existingOut)
	existing.Name = "existing"
	joining := newSyncTestClient(1, 101, joinOut)
	joining.Name = "joining"

	clients := &ClientManager{
		clients:    []*Client{existing, joining},
		broadcasts: utils.NewTopic[[]P.Message](),
	}

	server := &Server{
		Config: &Config{},
		State: &State{
			Clock:      syncTestClock{},
			GameMode:   syncTestMode{},
			Map:        "complex",
			MasterMode: mastermode.Auth,
		},
		Clients: clients,
	}

	server.SendWelcome(joining)

	packet := <-joinOut
	initIndex := -1
	resumeIndex := -1
	for index, message := range packet.Messages {
		switch message.(type) {
		case P.InitClient:
			if initIndex < 0 {
				initIndex = index
			}
		case P.Resume:
			resumeIndex = index
		}
	}

	if initIndex < 0 {
		t.Fatal("welcome snapshot did not include existing player's InitClient")
	}
	if resumeIndex < 0 {
		t.Fatal("welcome snapshot did not include Resume state")
	}
	if initIndex > resumeIndex {
		t.Fatalf("InitClient must be sent before Resume so late joiners can bind entity state: init=%d resume=%d", initIndex, resumeIndex)
	}
}

func TestInformOthersOfJoinSendsVisibleHumanSnapshot(t *testing.T) {
	existingOut := make(chan ServerPacket, 4)
	joiningOut := make(chan ServerPacket, 4)

	existing := newSyncTestClient(0, 100, existingOut)
	joining := newSyncTestClient(1, 101, joiningOut)

	clients := &ClientManager{
		clients:    []*Client{existing, joining},
		broadcasts: utils.NewTopic[[]P.Message](),
	}

	clients.InformOthersOfJoin(joining)

	packet := <-existingOut
	if len(packet.Messages) < 2 {
		t.Fatalf("expected InitClient and SpawnState, got %d messages", len(packet.Messages))
	}
	if _, ok := packet.Messages[0].(P.InitClient); !ok {
		t.Fatalf("first join packet must identify the player, got %T", packet.Messages[0])
	}
	if _, ok := packet.Messages[1].(P.SpawnState); !ok {
		t.Fatalf("second join packet must make the player visible, got %T", packet.Messages[1])
	}
}

func TestInformOthersOfJoinSendsVisibleBotSnapshotAbove127(t *testing.T) {
	ownerOut := make(chan ServerPacket, 4)
	otherOut := make(chan ServerPacket, 4)

	owner := newSyncTestClient(0, 100, ownerOut)
	other := newSyncTestClient(1, 101, otherOut)
	bot := NewBot(128, owner, 70, 2, "Vector")
	bot.State = playerstate.Alive
	bot.LifeSequence = 5

	clients := &ClientManager{
		clients:    []*Client{owner, other, bot},
		broadcasts: utils.NewTopic[[]P.Message](),
	}

	clients.InformOthersOfJoin(bot)

	for name, out := range map[string]chan ServerPacket{"owner": ownerOut, "other": otherOut} {
		packet := <-out
		if len(packet.Messages) < 2 {
			t.Fatalf("%s expected InitAI and SpawnState, got %d messages", name, len(packet.Messages))
		}
		if _, ok := packet.Messages[0].(P.InitAI); !ok {
			t.Fatalf("%s first bot packet must identify the AI, got %T", name, packet.Messages[0])
		}
		if _, ok := packet.Messages[1].(P.SpawnState); !ok {
			t.Fatalf("%s second bot packet must make the AI visible, got %T", name, packet.Messages[1])
		}
	}
}
