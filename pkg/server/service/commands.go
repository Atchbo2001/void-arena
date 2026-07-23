package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cfoust/sour/pkg/game"
	"github.com/cfoust/sour/pkg/game/commands"
	"github.com/cfoust/sour/pkg/game/constants"
	"github.com/cfoust/sour/pkg/gameserver/protocol/gamemode"
	"github.com/cfoust/sour/pkg/server/ingress"
	"github.com/cfoust/sour/pkg/server/servers"

	"github.com/repeale/fp-go/option"
	"github.com/rs/zerolog/log"
)

func (server *Cluster) GivePrivateMatchHelp(ctx context.Context, user *User, gameServer *servers.GameServer) {
	tick := time.NewTicker(30 * time.Second)

	message := fmt.Sprintf("This is your private server. Have other players join by saying '#join %s' in any Sour server.", gameServer.Id)

	if user.Connection.Type() == ingress.ClientTypeWS {
		message = fmt.Sprintf("This is your private server. Have other players join by saying '#join %s' in any Sour server or by sending the link in your URL bar. (We also copied it for you!)", gameServer.Id)
	}

	sessionContext := user.ServerSessionContext()

	for {
		gameServer.Mutex.Lock()
		numClients := gameServer.NumClients()
		gameServer.Mutex.Unlock()

		if numClients < 2 {
			user.Message(message)
		} else {
			return
		}

		select {
		case <-sessionContext.Done():
			return
		case <-tick.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}

type CreateParams struct {
	Map             opt.Option[string]
	Preset          opt.Option[string]
	Mode            opt.Option[int]
	Listed          bool
	Password        string
	RequirePassword bool
	Bots            int
	BotSkill        int
	MatchLength     int
	Title           string
}

func (server *Cluster) inferCreateParams(args []string) (*CreateParams, error) {
	params := CreateParams{
		Listed:      false,
		Bots:        0,
		BotSkill:    70,
		MatchLength: 300,
	}

	decodeValue := func(value string) string {
		decoded, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			return value
		}
		return string(decoded)
	}

	for _, arg := range args {
		if key, value, ok := strings.Cut(arg, "="); ok {
			switch key {
			case "visibility":
				switch value {
				case "public":
					params.Listed = true
					params.Password = ""
				case "password":
					params.Listed = true
					params.RequirePassword = true
				case "unlisted":
					params.Listed = false
				default:
					return nil, fmt.Errorf("invalid visibility")
				}
				continue
			case "password":
				params.Password = decodeValue(value)
				continue
			case "bots":
				parsed, err := strconv.Atoi(value)
				if err != nil || parsed < 0 || parsed > 12 {
					return nil, fmt.Errorf("bots must be between 0 and 12")
				}
				params.Bots = parsed
				continue
			case "skill":
				parsed, err := strconv.Atoi(value)
				if err != nil || parsed < 1 || parsed > 101 {
					return nil, fmt.Errorf("bot skill must be between 1 and 101")
				}
				params.BotSkill = parsed
				continue
			case "duration":
				parsed, err := strconv.Atoi(value)
				if err != nil || parsed < 120 || parsed > 1800 {
					return nil, fmt.Errorf("round length must be between 120 and 1800 seconds")
				}
				params.MatchLength = parsed
				continue
			case "title":
				params.Title = strings.TrimSpace(decodeValue(value))
				if len(params.Title) > 40 {
					params.Title = params.Title[:40]
				}
				continue
			}
		}
		mode := constants.GetModeNumber(arg)
		if opt.IsSome(mode) {
			params.Mode = mode
			continue
		}

		map_ := server.servers.Maps.FindMap(arg)
		if map_ != nil {
			params.Map = opt.Some(arg)
			continue
		}

		preset := server.servers.FindPreset(arg, false)
		if opt.IsSome(preset) {
			params.Preset = opt.Some(preset.Value.Name)
			continue
		}

		return nil, fmt.Errorf("argument '%s' neither corresponded to a map nor a game mode", arg)
	}

	if params.RequirePassword && len(params.Password) < 3 {
		return nil, fmt.Errorf("password-protected games require at least 3 characters")
	}
	return &params, nil
}

func (server *Cluster) CreateGame(ctx context.Context, params *CreateParams, user *User) error {
	logger := user.Logger()
	server.createMutex.Lock()
	defer server.createMutex.Unlock()

	creatorKey := fmt.Sprintf("user:%d", user.Id)
	lastCreate, hasLastCreate := server.lastCreate[creatorKey]
	if hasLastCreate && (time.Now().Sub(lastCreate)) < CREATE_SERVER_COOLDOWN {
		return errors.New("too soon since last server create")
	}

	existingServer, hasExistingServer := server.hostServers[creatorKey]
	if hasExistingServer {
		server.servers.RemoveServer(existingServer)
	}

	logger.Info().Msg("starting server")

	presetName := ""
	if opt.IsSome(params.Preset) {
		presetName = params.Preset.Value
	}

	gameServer, err := server.servers.NewServer(server.serverCtx, presetName, false)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create server")
		return errors.New("failed to create server")
	}

	logger = logger.With().Str("server", gameServer.Reference()).Logger()

	gameServer.Temporary = true
	gameServer.Config.MatchLength = params.MatchLength
	gameServer.Config.BotCount = params.Bots
	gameServer.Config.BotSkill = params.BotSkill
	gameServer.SetAccess(params.Listed, params.Password)
	if params.Title != "" {
		gameServer.SetDescription(fmt.Sprintf("%s [%s]", params.Title, gameServer.Id))
	}

	mode := int32(params.Mode.Value)
	if opt.IsSome(params.Mode) && !gamemode.Valid(gamemode.ID(mode)) {
		return fmt.Errorf("game mode not yet supported")
	}

	if opt.IsSome(params.Mode) && opt.IsSome(params.Map) {
		gameServer.ChangeMap(mode, params.Map.Value)
	} else if opt.IsSome(params.Mode) {
		gameServer.SetMode(mode)
	} else if opt.IsSome(params.Map) {
		gameServer.SetMap(params.Map.Value)
	}

	server.lastCreate[creatorKey] = time.Now()
	server.hostServers[creatorKey] = gameServer

	connected, err := user.ConnectToServer(gameServer, "", false, true)
	go server.GivePrivateMatchHelp(server.serverCtx, user, user.Server)

	go func() {
		ctx, cancel := context.WithTimeout(user.Connection.Session().Ctx(), time.Second*10)
		defer cancel()

		select {
		case status := <-connected:
			if !status {
				return
			}

			user.ServerClient.GrantMaster()
		case <-ctx.Done():
			return
		}
	}()

	return nil
}

func (s *Cluster) runCommand(ctx context.Context, user *User, command string) error {
	contexts := make([]commands.Commandable, 0)

	args := strings.Split(command, " ")
	if len(args) == 0 {
		return fmt.Errorf("command cannot be empty")
	}

	// First check cluster commands
	if s.commands.CanHandle(args) {
		return s.commands.Handle(ctx, user, args)
	}

	contexts = append(contexts, s.commands)

	// TODO then do space

	server := user.GetServer()
	if server != nil && server.Commands.CanHandle(args) {
		client := server.Clients.GetClientByID(uint32(user.Id))
		if client != nil {
			return server.Commands.Handle(ctx, client, args)
		}
	}

	if server != nil {
		contexts = append(contexts, server.Commands)
	}

	// Then help
	first := args[0]
	if first != "help" && first != "?" {
		return fmt.Errorf("unrecognized command")
	}

	helpArgs := args[1:]
	if len(helpArgs) == 0 {
		user.RawMessage("available commands: (say '#help [command]' for more information)")
		for _, commandable := range contexts {
			user.RawMessage(commandable.Help())
		}
		return nil
	}

	// Help for a specific command
	for _, commandable := range contexts {
		helpString, err := commandable.GetHelp(helpArgs)
		if err == nil {
			user.RawMessage(helpString)
			return nil
		}
	}

	// Did not match anything
	return fmt.Errorf("could not find help for command")
}

func (s *Cluster) runCommandWithTimeout(ctx context.Context, user *User, command string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)

	resultChannel := make(chan error)
	defer cancel()

	go func() {
		resultChannel <- s.runCommand(ctx, user, command)
	}()

	select {
	case result := <-resultChannel:
		return result
	case <-ctx.Done():
		return fmt.Errorf("command timed out")
	}
}

func (s *Cluster) registerCommands() {
	goCommand := commands.Command{
		Name:        "go",
		Aliases:     []string{"join"},
		ArgFormat:   "[name|id|alias] [password]",
		Description: "move to a server by name, id, or alias",
		Callback: func(ctx context.Context, user *User, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("missing server name")
			}
			target := args[0]
			password := ""
			if len(args) > 1 {
				password = args[1]
				if decoded, err := base64.RawURLEncoding.DecodeString(password); err == nil {
					password = string(decoded)
				}
			}
			for _, gameServer := range s.servers.Servers {
				if !gameServer.IsReference(target) {
					continue
				}
				if !gameServer.CheckPassword(password) {
					return fmt.Errorf("incorrect password")
				}

				_, err := user.Connect(gameServer)
				return err
			}

			return fmt.Errorf("could not find server '%s'", target)
		},
	}

	createGameCommand := commands.Command{
		Name:        "creategame",
		ArgFormat:   "[mode] [map] [visibility=public|password|unlisted] [password=value] [bots=0..12] [skill=1..101] [duration=seconds]",
		Description: "create a private game for you and your friends",
		Callback: func(ctx context.Context, user *User, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("you must provide at least one argument")
			}

			params, err := s.inferCreateParams(args)
			if err != nil {
				return err
			}

			return s.CreateGame(ctx, params, user)
		},
	}

	duelCommand := commands.Command{
		Name:        "duel",
		ArgFormat:   "[ffa|insta]",
		Aliases:     []string{"queue"},
		Description: "queue for 1v1 matchmaking",
		Callback: func(ctx context.Context, user *User, duelType string) error {
			err := s.matches.Queue(user, duelType)
			if err != nil {
				// Theoretically, there might also just not be a default, but whatever.
				return fmt.Errorf("duel type '%s' does not exist", duelType)
			}

			return nil
		},
	}

	stopDuelCommand := commands.Command{
		Name:        "stopduel",
		Description: "unqueue from 1v1 matchmaking",
		Callback: func(ctx context.Context, user *User) {
			s.matches.Dequeue(user)
		},
	}

	err := s.commands.Register(
		goCommand,
		createGameCommand,
		duelCommand,
		stopDuelCommand,
	)

	if err != nil {
		log.Fatal().Err(err).Msg("failed to register cluster command")
	}
}

func (s *Cluster) HandleCommand(ctx context.Context, user *User, command string) {
	err := s.runCommandWithTimeout(ctx, user, command)
	logger := user.Logger()
	if err != nil {
		logger.Error().Err(err).Msgf("user command failed: %s", command)
		user.Message(game.Red(fmt.Sprintf("command failed: %s", err.Error())))
		return
	}
}
