package ggpo

type PlayerHandle int

type PlayerType int

const (
	PlayerTypeLocal PlayerType = iota
	PlayerTypeRemote
	PlayerTypeSpectator
)

const InvalidHandle int = -1

type Player struct {
	Size       int
	PlayerType PlayerType
	PlayerNum  int
	Remote     RemotePlayer
}

func NewLocalPlayer(size int, playerNum int) Player {
	return Player{
		Size:       size,
		PlayerNum:  playerNum,
		PlayerType: PlayerTypeLocal}
}

func NewRemotePlayer(size int, playerNum int, ipAddress string, port int) Player {
	return Player{
		Size:       size,
		PlayerNum:  playerNum,
		PlayerType: PlayerTypeRemote,
		Remote: RemotePlayer{
			IpAddress: ipAddress,
			Port:      port},
	}
}
func NewSpectatorPlayer(size int, ipAddress string, port int) Player {
	return Player{
		Size:       size,
		PlayerType: PlayerTypeSpectator,
		Remote: RemotePlayer{
			IpAddress: ipAddress,
			Port:      port},
	}
}

type RemotePlayer struct {
	IpAddress string
	Port      int
}

type LocalEndpoint struct {
	playerNum int
}
