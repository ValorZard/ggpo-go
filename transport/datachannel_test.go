package transport_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ikemen-engine/ggpo/internal/messages"
	"github.com/ikemen-engine/ggpo/transport"
)

// msgPipe is an in-memory, message-oriented io.ReadWriteCloser: every Write
// on one end is returned by exactly one Read on the other end, mimicking a
// detached WebRTC data channel.
type msgPipe struct {
	in        chan []byte
	out       chan []byte
	closed    chan struct{}
	closeOnce sync.Once
}

func newMsgPipePair() (*msgPipe, *msgPipe) {
	aToB := make(chan []byte, 64)
	bToA := make(chan []byte, 64)
	closed := make(chan struct{})
	a := &msgPipe{in: bToA, out: aToB, closed: closed}
	b := &msgPipe{in: aToB, out: bToA, closed: closed}
	return a, b
}

func (p *msgPipe) Read(buf []byte) (int, error) {
	select {
	case msg := <-p.in:
		return copy(buf, msg), nil
	case <-p.closed:
		return 0, errors.New("pipe closed")
	}
}

func (p *msgPipe) Write(buf []byte) (int, error) {
	msg := make([]byte, len(buf))
	copy(msg, buf)
	select {
	case p.out <- msg:
		return len(buf), nil
	case <-p.closed:
		return 0, errors.New("pipe closed")
	}
}

func (p *msgPipe) Close() error {
	p.closeOnce.Do(func() { close(p.closed) })
	return nil
}

func TestDataChannelRoutesMessages(t *testing.T) {
	local1, remote1 := newMsgPipePair()
	local2, remote2 := newMsgPipePair()

	dc := transport.NewDataChannel()
	dc.AddPeerChannel("player", 1, local1)
	dc.AddPeerChannel("player", 2, local2)
	defer dc.Close()

	messageChan := make(chan transport.MessageChannelItem, 16)
	go dc.Read(messageChan)

	// A message written by remote peer 2 arrives tagged with peer 2's address.
	sent := messages.NewUDPMessage(messages.SyncRequestMsg)
	sent.SetHeader(42, 7)
	if _, err := remote2.Write(sent.ToBytes()); err != nil {
		t.Fatalf("writing to pipe failed: %s", err)
	}

	select {
	case item := <-messageChan:
		if item.Peer.Ip != "player" || item.Peer.Port != 2 {
			t.Errorf("message attributed to %s:%d, want player:2", item.Peer.Ip, item.Peer.Port)
		}
		if item.Message.Type() != messages.SyncRequestMsg {
			t.Errorf("received message type %v, want SyncRequestMsg", item.Message.Type())
		}
		if item.Message.Header().Magic != 42 {
			t.Errorf("received magic %d, want 42", item.Message.Header().Magic)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for incoming message")
	}

	// SendTo routes to the channel registered for that (ip, port) key.
	dc.SendTo(sent, "player", 1)
	buf := make([]byte, transport.MaxUDPPacketSize)
	n, err := remote1.Read(buf)
	if err != nil {
		t.Fatalf("reading from pipe failed: %s", err)
	}
	got, err := messages.DecodeMessageBinary(buf[:n])
	if err != nil {
		t.Fatalf("decoding sent message failed: %s", err)
	}
	if got.Type() != messages.SyncRequestMsg || got.Header().Magic != 42 {
		t.Errorf("received %v with magic %d, want SyncRequestMsg with magic 42", got.Type(), got.Header().Magic)
	}

	// Nothing should have arrived at the other remote.
	select {
	case msg := <-remote2.in:
		t.Errorf("unexpected message delivered to remote2: %v", msg)
	default:
	}

	// Sending to an unregistered peer drops the message rather than panicking.
	dc.SendTo(sent, "unknown", 9)
}

func TestDataChannelAddPeerAfterRead(t *testing.T) {
	dc := transport.NewDataChannel()
	defer dc.Close()

	messageChan := make(chan transport.MessageChannelItem, 16)
	go dc.Read(messageChan)

	// Registering a channel after Read has started must still receive.
	local, remote := newMsgPipePair()
	dc.AddPeerChannel("late", 1, local)

	sent := messages.NewUDPMessage(messages.KeepAliveMsg)
	if _, err := remote.Write(sent.ToBytes()); err != nil {
		t.Fatalf("writing to pipe failed: %s", err)
	}

	select {
	case item := <-messageChan:
		if item.Peer.Ip != "late" || item.Peer.Port != 1 {
			t.Errorf("message attributed to %s:%d, want late:1", item.Peer.Ip, item.Peer.Port)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message from late-registered channel")
	}
}
