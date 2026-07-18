package webrtcconn_test

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ikemen-engine/ggpo"
	"github.com/ikemen-engine/ggpo/internal/mocks"
	"github.com/ikemen-engine/ggpo/signaling"
	"github.com/ikemen-engine/ggpo/webrtcconn"
)

// establishPair connects a host and a joiner through an in-process signaling
// server and returns both ends of the resulting data channel.
func establishPair(t *testing.T) (hostChannel, joinChannel io.ReadWriteCloser) {
	t.Helper()

	server := httptest.NewServer(signaling.NewServer().Handler())
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	// No ICE servers: host candidates are enough on localhost.
	hostDialer := webrtcconn.NewDialer(server.URL)
	hostDialer.PollInterval = 50 * time.Millisecond
	joinDialer := webrtcconn.NewDialer(server.URL)
	joinDialer.PollInterval = 50 * time.Millisecond

	lobby, err := hostDialer.HostLobby(ctx)
	if err != nil {
		t.Fatalf("hosting lobby failed: %s", err)
	}

	type joinResult struct {
		channel io.ReadWriteCloser
		id      int
		err     error
	}
	joinChan := make(chan joinResult, 1)
	go func() {
		channel, id, err := joinDialer.Join(ctx, lobby.ID)
		joinChan <- joinResult{channel, id, err}
	}()

	hostSide, playerID, err := lobby.Accept(ctx)
	if err != nil {
		t.Fatalf("accepting player failed: %s", err)
	}
	if playerID != 1 {
		t.Errorf("accepted player has ID %d, want 1", playerID)
	}
	if err := lobby.Delete(ctx); err != nil {
		t.Errorf("deleting lobby failed: %s", err)
	}

	joined := <-joinChan
	if joined.err != nil {
		t.Fatalf("joining lobby failed: %s", joined.err)
	}
	if joined.id != 1 {
		t.Errorf("joiner got player ID %d, want 1", joined.id)
	}

	t.Cleanup(func() {
		hostSide.Close()
		joined.channel.Close()
	})
	return hostSide, joined.channel
}

func TestWebRTCChannelRoundTrip(t *testing.T) {
	hostChannel, joinChannel := establishPair(t)

	payload := []byte{1, 2, 3, 4, 5}
	if _, err := joinChannel.Write(payload); err != nil {
		t.Fatalf("writing to channel failed: %s", err)
	}
	buf := make([]byte, 1024)
	n, err := hostChannel.Read(buf)
	if err != nil {
		t.Fatalf("reading from channel failed: %s", err)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Errorf("host received %v, want %v", buf[:n], payload)
	}

	if _, err := hostChannel.Write(payload); err != nil {
		t.Fatalf("writing to channel failed: %s", err)
	}
	n, err = joinChannel.Read(buf)
	if err != nil {
		t.Fatalf("reading from channel failed: %s", err)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Errorf("joiner received %v, want %v", buf[:n], payload)
	}
}

// TestGGPOSessionOverWebRTC runs two full GGPO peers over a real WebRTC data
// channel connection: both synchronize, then exchange inputs for a number of
// frames and end up with identical synchronized inputs.
func TestGGPOSessionOverWebRTC(t *testing.T) {
	hostChannel, joinChannel := establishPair(t)

	numPlayers := 2
	inputSize := 4

	// FakeSessionWithBackend drives rollbacks through the backend, which the
	// asynchronous real network makes likely (inputs arrive after prediction).
	session1 := mocks.NewFakeSessionWithBackend()
	peer1, transport1 := ggpo.NewDataChannelPeer(&session1, numPlayers, inputSize)
	session1.SetBackend(&peer1)
	transport1.AddPeerChannel("player", 2, hostChannel)
	// Choosing UDP vs data channels happens at creation time; a bare
	// InitializeConnection must not clobber the data channel transport.
	peer1.InitializeConnection()

	session2 := mocks.NewFakeSessionWithBackend()
	peer2, transport2 := ggpo.NewDataChannelPeer(&session2, numPlayers, inputSize)
	session2.SetBackend(&peer2)
	transport2.AddPeerChannel("player", 1, joinChannel)

	player1Local := ggpo.NewLocalPlayer(20, 1)
	player2Remote := ggpo.NewRemotePlayer(20, 2, "player", 2)
	var p1LocalHandle, p1RemoteHandle ggpo.PlayerHandle
	if err := peer1.AddPlayer(&player1Local, &p1LocalHandle); err != nil {
		t.Fatalf("adding local player to peer 1 failed: %s", err)
	}
	if err := peer1.AddPlayer(&player2Remote, &p1RemoteHandle); err != nil {
		t.Fatalf("adding remote player to peer 1 failed: %s", err)
	}

	player1Remote := ggpo.NewRemotePlayer(20, 1, "player", 1)
	player2Local := ggpo.NewLocalPlayer(20, 2)
	var p2RemoteHandle, p2LocalHandle ggpo.PlayerHandle
	if err := peer2.AddPlayer(&player1Remote, &p2RemoteHandle); err != nil {
		t.Fatalf("adding remote player to peer 2 failed: %s", err)
	}
	if err := peer2.AddPlayer(&player2Local, &p2LocalHandle); err != nil {
		t.Fatalf("adding local player to peer 2 failed: %s", err)
	}

	peer1.Start()
	peer2.Start()
	defer peer1.Close()
	defer peer2.Close()

	// The same constant input on both sides keeps the toy session
	// deterministic and rollback-free, like the UDP backend tests.
	inputBytes := []byte{1, 2, 3, 4}
	targetFrames := 20
	frames1, frames2 := 0, 0

	runFrame := func(peer *ggpo.Peer, session *mocks.FakeSessionWithBackend, handle ggpo.PlayerHandle) bool {
		peer.Idle(0)
		if err := peer.AddLocalInput(handle, inputBytes, inputSize); err != nil {
			return false // still synchronizing
		}
		var disconnectFlags int
		vals, err := peer.SyncInput(&disconnectFlags)
		if err != nil {
			return false
		}
		session.Game.UpdateByInputs(vals)
		peer.AdvanceFrame(ggpo.DefaultChecksum)
		return true
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && (frames1 < targetFrames || frames2 < targetFrames) {
		if frames1 < targetFrames && runFrame(&peer1, &session1, p1LocalHandle) {
			frames1++
		}
		if frames2 < targetFrames && runFrame(&peer2, &session2, p2LocalHandle) {
			frames2++
		}
		time.Sleep(2 * time.Millisecond)
	}

	if frames1 < targetFrames || frames2 < targetFrames {
		t.Fatalf("peers did not run enough frames before the deadline: peer1=%d peer2=%d, want %d",
			frames1, frames2, targetFrames)
	}

	var disconnectFlags int
	vals1, err := peer1.SyncInput(&disconnectFlags)
	if err != nil {
		t.Fatalf("synchronizing input on peer 1 failed: %s", err)
	}
	vals2, err := peer2.SyncInput(&disconnectFlags)
	if err != nil {
		t.Fatalf("synchronizing input on peer 2 failed: %s", err)
	}
	if len(vals1) != len(vals2) {
		t.Fatalf("synchronized input lengths differ: %d vs %d", len(vals1), len(vals2))
	}
	for i := range vals1 {
		if !bytes.Equal(vals1[i], vals2[i]) {
			t.Errorf("synchronized input %d differs: peer1=%v peer2=%v", i+1, vals1[i], vals2[i])
		}
	}
}
