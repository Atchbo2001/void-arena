package gameserver

type Config struct {
	MaxClients        int
	MatchLength       int
	DefaultGameSpeed  int
	DefaultMode       string
	DefaultMap        string
	Maps              []string
	PasswordProtected bool
	BotCount          int
	BotSkill          int
}
