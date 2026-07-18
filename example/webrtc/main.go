// Command webrtc is a headless two-player GGPO demo where the transport is
// chosen at session creation: raw UDP or WebRTC data channels.
//
// WebRTC mode needs the signaling server running somewhere both players can
// reach:
//
//	go run github.com/ikemen-engine/ggpo/cmd/signaling -addr :3000
//
// Then, in one terminal, host a lobby (this prints the lobby ID):
//
//	go run ./example/webrtc -transport webrtc -host
//
// And in another terminal join it:
//
//	go run ./example/webrtc -transport webrtc -join <lobbyID>
//
// UDP mode is the classic address-based setup:
//
//	go run ./example/webrtc -transport udp -player 1 -local-port 6000 -remote 127.0.0.1:6001
//	go run ./example/webrtc -transport udp -player 2 -local-port 6001 -remote 127.0.0.1:6000
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ikemen-engine/ggpo"
	"github.com/ikemen-engine/ggpo/transport"
	"github.com/ikemen-engine/ggpo/webrtcconn"
	"github.com/pion/webrtc/v4"
)

const (
	numPlayers = 2
	inputSize  = 4
	frameDelay = 2
)

// Game is a deterministic toy game: each player's input moves their counter.
type Game struct {
	Positions [numPlayers]int32
	Frame     int32
}

func (g *Game) Update(inputs [][]byte) {
	for i := 0; i < numPlayers && i < len(inputs); i++ {
		if len(inputs[i]) >= inputSize {
			g.Positions[i] += int32(binary.LittleEndian.Uint32(inputs[i]))
		}
	}
	g.Frame++
}

func (g *Game) Checksum() uint32 {
	return uint32(g.Positions[0])*2654435761 ^ uint32(g.Positions[1])*40503 ^ uint32(g.Frame)
}

// GameSession implements ggpo.Session.
type GameSession struct {
	game       Game
	saveStates map[int]Game
	backend    ggpo.Backend
	running    bool
}

func NewGameSession() *GameSession {
	return &GameSession{saveStates: make(map[int]Game)}
}

func (s *GameSession) SaveGameState(stateID int) int {
	s.saveStates[stateID] = s.game
	return int(s.game.Checksum())
}

func (s *GameSession) LoadGameState(stateID int) {
	s.game = s.saveStates[stateID]
}

func (s *GameSession) AdvanceFrame(flags int) {
	var disconnectFlags int
	inputs, err := s.backend.SyncInput(&disconnectFlags)
	if err == nil {
		s.game.Update(inputs)
		s.backend.AdvanceFrame(s.game.Checksum())
	}
}

func (s *GameSession) OnEvent(info *ggpo.Event) {
	switch info.Code {
	case ggpo.EventCodeRunning:
		fmt.Println("Synchronized, game running.")
		s.running = true
	case ggpo.EventCodeSynchronizingWithPeer:
		fmt.Printf("Synchronizing with peer (%d/%d)...\n", info.Count, info.Total)
	case ggpo.EventCodeDisconnectedFromPeer:
		fmt.Println("Disconnected from peer.")
		s.running = false
	case ggpo.EventCodeDesync:
		fmt.Printf("DESYNC at frame %d: local %d remote %d\n",
			info.NumFrameOfDesync, info.LocalChecksum, info.RemoteChecksum)
	}
}

func main() {
	transportFlag := flag.String("transport", "webrtc", "transport to use: webrtc or udp")
	signalingURL := flag.String("signaling", "http://127.0.0.1:3000", "signaling server URL (webrtc)")
	host := flag.Bool("host", false, "host a lobby (webrtc)")
	join := flag.String("join", "", "lobby ID to join (webrtc)")
	stun := flag.String("stun", "stun:stun.l.google.com:19302", "STUN server, empty for none (webrtc)")
	player := flag.Int("player", 1, "local player number, 1 or 2 (udp)")
	localPort := flag.Int("local-port", 6000, "local UDP port (udp)")
	remote := flag.String("remote", "127.0.0.1:6001", "remote player address (udp)")
	frames := flag.Int("frames", 600, "number of frames to run")
	flag.Parse()

	session := NewGameSession()

	var peer ggpo.Peer
	var localPlayer, remotePlayer ggpo.Player

	switch *transportFlag {
	case "webrtc":
		dialer := webrtcconn.NewDialer(*signalingURL)
		if *stun != "" {
			dialer.ICEServers = []webrtc.ICEServer{{URLs: []string{*stun}}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// The peer is created against a data channel transport instead of raw
		// UDP; the channel itself comes from the signaling exchange below.
		var dataChannel *transport.DataChannel
		peer, dataChannel = ggpo.NewDataChannelPeer(session, numPlayers, inputSize)

		var localPlayerNum int
		if *host {
			lobby, err := dialer.HostLobby(ctx)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Hosting lobby, share this ID: %s\n", lobby.ID)
			channel, playerID, err := lobby.Accept(ctx)
			if err != nil {
				log.Fatal(err)
			}
			lobby.Delete(ctx)
			localPlayerNum = 1
			// (ip, port) is just a routing key for data channels; use the
			// remote's player number.
			dataChannel.AddPeerChannel("player", 2, channel)
			fmt.Printf("Player %d connected.\n", playerID)
		} else {
			if *join == "" {
				log.Fatal("webrtc mode needs -host or -join <lobbyID>")
			}
			channel, playerID, err := dialer.Join(ctx, *join)
			if err != nil {
				log.Fatal(err)
			}
			localPlayerNum = 2
			dataChannel.AddPeerChannel("player", 1, channel)
			fmt.Printf("Joined lobby as player %d.\n", playerID)
		}

		localPlayer = ggpo.NewLocalPlayer(20, localPlayerNum)
		remotePlayer = ggpo.NewRemotePlayer(20, 3-localPlayerNum, "player", 3-localPlayerNum)
	case "udp":
		if *player != 1 && *player != 2 {
			log.Fatal("-player must be 1 or 2")
		}
		remoteIP, remotePortStr, err := net.SplitHostPort(*remote)
		if err != nil {
			log.Fatal("please pass -remote as ip:port")
		}
		remotePort, err := strconv.Atoi(remotePortStr)
		if err != nil {
			log.Fatal("please pass an integer port in -remote")
		}
		peer = ggpo.NewPeer(session, *localPort, numPlayers, inputSize)
		peer.InitializeConnection() // raw UDP on -local-port

		localPlayer = ggpo.NewLocalPlayer(20, *player)
		remotePlayer = ggpo.NewRemotePlayer(20, 3-*player, remoteIP, remotePort)
	default:
		log.Fatalf("unknown -transport %q, expected webrtc or udp", *transportFlag)
	}

	session.backend = &peer

	var localHandle, remoteHandle ggpo.PlayerHandle
	if err := peer.AddPlayer(&localPlayer, &localHandle); err != nil {
		log.Fatal(err)
	}
	if err := peer.AddPlayer(&remotePlayer, &remoteHandle); err != nil {
		log.Fatal(err)
	}
	peer.SetDisconnectTimeout(5000)
	peer.SetDisconnectNotifyStart(1000)
	peer.SetFrameDelay(localHandle, frameDelay)
	peer.Start()
	defer peer.Close()

	fmt.Printf("Running %d frames over %s...\n", *frames, *transportFlag)
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	frame := 0
	for range ticker.C {
		peer.Idle(0)

		// A deterministic synthetic "input" so both sides can verify state.
		input := make([]byte, inputSize)
		binary.LittleEndian.PutUint32(input, uint32(frame%5))
		if err := peer.AddLocalInput(localHandle, input, inputSize); err != nil {
			continue // still synchronizing (or in rollback), keep idling
		}

		var disconnectFlags int
		inputs, err := peer.SyncInput(&disconnectFlags)
		if err != nil {
			continue
		}
		session.game.Update(inputs)
		peer.AdvanceFrame(session.game.Checksum())

		frame++
		if frame%60 == 0 {
			fmt.Printf("frame %4d positions=%v checksum=%08x\n",
				frame, session.game.Positions, session.game.Checksum())
		}
		if frame >= *frames {
			break
		}
	}

	// Keep idling briefly so late remote inputs arrive and rollbacks correct
	// any speculative frames; both sides then print the same final state.
	for i := 0; i < 60; i++ {
		peer.Idle(0)
		time.Sleep(time.Second / 60)
	}

	fmt.Printf("Done. Final state: positions=%v frame=%d checksum=%08x\n",
		session.game.Positions, session.game.Frame, session.game.Checksum())
	os.Exit(0)
}
