package webrtc

import (
	"io"
	"strconv"
	"sync"

	"github.com/ikemen-engine/ggpo/transport"
)

// maxMessageSize bounds a single decoded packet. WebRTC data channels are
// message-oriented, so one Read yields one whole packet; this only needs to be
// large enough for the biggest GGPO packet.
const maxMessageSize = 4096

// Transport adapts a set of WebRTC data channels to transport.Connection, so
// the GGPO protocol layer can drive a WebRTC session exactly as it drives UDP.
//
// The protocol layer addresses peers by (ip, port), but WebRTC has no such
// addresses. Each established channel is therefore registered with AddPeer
// under a synthetic address, and the caller reuses that same address when it
// adds the player to the session (Backend.AddPlayer). A typical host flow:
//
//	tr := webrtc.NewTransport()
//	backend.InitializeConnection(tr)   // protocol layer will call tr.Read
//	ch, playerID, _ := lobby.Accept(ctx)
//	ip, port := addrForPlayer(playerID)
//	tr.AddPeer(ip, port, ch)
//	backend.AddPlayer(&Player{Remote: {IpAddress: ip, Port: port}, ...}, &handle)
type Transport struct {
	mu        sync.Mutex
	peers     map[string]*peer
	msgChan   chan transport.MessageChannelItem
	done      chan struct{}
	closeOnce sync.Once
}

type peer struct {
	channel io.ReadWriteCloser
	ip      string
	port    int
	reading bool
}

func NewTransport() *Transport {
	return &Transport{
		peers: make(map[string]*peer),
		done:  make(chan struct{}),
	}
}

func addrKey(ip string, port int) string {
	return ip + ":" + strconv.Itoa(port)
}

// AddPeer registers an established data channel under the synthetic address the
// session will use for this player. It is safe to call before or after Read: a
// reader goroutine starts as soon as both the channel and the destination for
// decoded messages (set by Read) are known.
// TODO: we don't need to do this, just replace this with the player's id as assigned from the lobby
func (t *Transport) AddPeer(ip string, port int, channel io.ReadWriteCloser) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p := &peer{channel: channel, ip: ip, port: port}
	t.peers[addrKey(ip, port)] = p
	if t.msgChan != nil {
		p.reading = true
		go t.readPeer(p)
	}
}

// SendTo serializes msg and writes it to the data channel registered for
// remoteIp:remotePort. Sends to an unregistered peer are dropped, mirroring a
// UDP send to an unreachable address.
func (t *Transport) SendTo(msg transport.Message, remoteIp string, remotePort int) {
	if msg == nil {
		return
	}
	t.mu.Lock()
	p, ok := t.peers[addrKey(remoteIp, remotePort)]
	t.mu.Unlock()
	if !ok {
		return
	}
	p.channel.Write(msg.ToBytes())
}

// Read records where decoded messages should be delivered, starts a reader for
// every peer registered so far, then blocks until Close. Run it in its own
// goroutine, the same way udp.Udp.Read is used.
func (t *Transport) Read(messageChan chan transport.MessageChannelItem) {
	t.mu.Lock()
	t.msgChan = messageChan
	for _, p := range t.peers {
		if !p.reading {
			p.reading = true
			go t.readPeer(p)
		}
	}
	t.mu.Unlock()
	<-t.done
}

// readPeer reads whole packets from one data channel until it closes, decoding
// each and forwarding it tagged with the peer's synthetic address.
func (t *Transport) readPeer(p *peer) {
	buf := make([]byte, maxMessageSize)
	for {
		n, err := p.channel.Read(buf)
		if err != nil {
			return // channel closed
		}
		if n <= 0 {
			continue
		}
		msg, err := transport.DecodeMessageBinary(buf[:n])
		if err != nil {
			continue // drop malformed packets, as UDP would
		}
		select {
		case t.msgChan <- transport.MessageChannelItem{
			Peer:    transport.PeerAddress{Ip: p.ip, Port: p.port},
			Message: msg,
			Length:  n,
		}:
		case <-t.done:
			return
		}
	}
}

// Close tears down every registered data channel and unblocks Read.
func (t *Transport) Close() {
	t.closeOnce.Do(func() {
		close(t.done)
		t.mu.Lock()
		for _, p := range t.peers {
			p.channel.Close()
		}
		t.mu.Unlock()
	})
}
