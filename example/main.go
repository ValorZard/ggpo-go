package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	//	"net/http"
	// _ "net/http/pprof"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ikemen-engine/ggpo"
)

type peerAddress struct {
	ip   string
	port int
}

// remotePeer is a player handle together with the UDP address to register it
// under.
type remotePeer struct {
	handle ggpo.PlayerHandle
	ip     string
	port   int
}

func getPeerAddress(address string) peerAddress {
	peerIPSlice := strings.Split(address, ":")
	if len(peerIPSlice) < 2 {
		panic("Please enter IP as ip:port")
	}
	peerPort, err := strconv.Atoi(peerIPSlice[1])
	if err != nil {
		panic("Please enter integer port")
	}
	return peerAddress{
		ip:   peerIPSlice[0],
		port: peerPort,
	}
}

func main() {

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) < 4 {
		panic("Must enter <port> <num players> ('local' |IP adress) ('local' |IP adress) currentPlayer or <port> <num players> spectate <host ip>:<host port>")
	}
	var localPort, numPlayers int
	var err error
	localPort, err = strconv.Atoi(argsWithoutProg[0])
	if err != nil {
		panic("Plase enter integer port")
	}

	numPlayers, err = strconv.Atoi(argsWithoutProg[1])
	if err != nil {
		panic("Please enter integer numPlayers")
	}

	// logFileName := ""
	// if len(argsWithoutProg) > 4 {
	// 	logFileName = "Player" + argsWithoutProg[4] + ".log"
	// } else {
	// 	logFileName = "Spectator.log"
	// }

	// f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR, 0666)
	// if err != nil {
	// 	panic(err)
	// }

	// // don't forget to close it
	// defer f.Close()
	// logger := log.New(f, "Logger:", log.Ldate|log.Ltime|log.Lshortfile)
	// ggpo.EnableLogs()
	// ggpo.SetLogger(logger)

	var game *Game
	if argsWithoutProg[2] == "spectate" {
		hostIp := argsWithoutProg[3]
		hostAddress := getPeerAddress(hostIp)
		game = GameInitSpectator(localPort, numPlayers, hostAddress.ip, hostAddress.port)
	} else {
		ipAddress := []string{argsWithoutProg[2], argsWithoutProg[3]}

		currentPlayer, err = strconv.Atoi(argsWithoutProg[4])
		if err != nil {
			panic("Please enter integer currentPlayer")
		}

		players := make([]ggpo.Player, ggpo.MaxPlayers+ggpo.MaxSpectators)
		var remotePeers []remotePeer
		var i int
		for i = 0; i < numPlayers; i++ {
			// the handle for each remote player is simply their player number
			handle := ggpo.PlayerHandle(i + 1)
			if ipAddress[i] == "local" {
				players[i] = ggpo.NewLocalPlayer(20, i+1)
			} else {
				remoteAddress := getPeerAddress(ipAddress[i])
				players[i] = ggpo.NewRemotePlayer(20, i+1, handle)
				remotePeers = append(remotePeers, remotePeer{handle: handle, ip: remoteAddress.ip, port: remoteAddress.port})
			}
		}

		offset := 5
		numSpectators := 0
		for offset < len(argsWithoutProg) {
			remoteAddress := getPeerAddress(argsWithoutProg[offset])
			// spectator handles start at 1000 to stay clear of player numbers
			handle := ggpo.PlayerHandle(1000 + numSpectators)
			players[i] = ggpo.NewSpectatorPlayer(20, handle)
			remotePeers = append(remotePeers, remotePeer{handle: handle, ip: remoteAddress.ip, port: remoteAddress.port})
			numSpectators++
			i++
			offset++
		}
		game = GameInit(localPort, numPlayers, players, numSpectators, remotePeers)
	}

	flag.Parse()
	start = time.Now().UnixMilli()
	next = start
	now = start
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}

}
