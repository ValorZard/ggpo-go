package transport

import (
	"io"
	"strconv"
	"sync"

	"github.com/ikemen-engine/ggpo/internal/messages"
	"github.com/ikemen-engine/ggpo/internal/util"
)

// DataChannel is a transport.Connection that runs GGPO messages over
// message-oriented io.ReadWriteCloser streams, such as detached Pion WebRTC
// data channels (see the webrtcconn package).
//
// Because GGPO addresses remote players by an (ip, port) pair, each peer's
// channel is registered under such a pair with AddPeerChannel. When using
// data channels the pair is only a routing key: it must match the address
// given to NewRemotePlayer for that player, but it does not need to be a
// real network address.
//
// Each Write on a registered channel must send exactly one message and each
// Read must return exactly one message, which is how detached WebRTC data
// channels behave.
type DataChannel struct {
	mutex       sync.Mutex
	peers       map[string]*peerChannel
	messageChan chan MessageChannelItem
	reading     bool
	done        chan struct{}
	closeOnce   sync.Once
}

type peerChannel struct {
	addr     peerAddress
	rwc      io.ReadWriteCloser
	sendChan chan messages.UDPMessage
}

func peerKey(ip string, port int) string {
	return ip + ":" + strconv.Itoa(port)
}

func NewDataChannel() *DataChannel {
	return &DataChannel{
		peers: make(map[string]*peerChannel),
		done:  make(chan struct{}),
	}
}

// AddPeerChannel registers the channel used to reach the remote peer known to
// GGPO as (ip, port). It may be called before or after the connection has been
// started; channels added after the fact begin receiving immediately.
func (d *DataChannel) AddPeerChannel(ip string, port int, rwc io.ReadWriteCloser) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	pc := &peerChannel{
		addr:     peerAddress{Ip: ip, Port: port},
		rwc:      rwc,
		sendChan: make(chan messages.UDPMessage, 256),
	}
	d.peers[peerKey(ip, port)] = pc

	go d.writeLoop(pc)
	if d.reading {
		go d.readLoop(pc)
	}
}

func (d *DataChannel) writeLoop(pc *peerChannel) {
	for {
		select {
		case msg := <-pc.sendChan:
			if _, err := pc.rwc.Write(msg.ToBytes()); err != nil {
				util.Log.Printf("DataChannel write error for %s:%d: %s\n", pc.addr.Ip, pc.addr.Port, err)
				return
			}
		case <-d.done:
			return
		}
	}
}

func (d *DataChannel) readLoop(pc *peerChannel) {
	recvBuf := make([]byte, MaxUDPPacketSize*2)
	for {
		n, err := pc.rwc.Read(recvBuf)
		if err != nil {
			util.Log.Printf("DataChannel read error for %s:%d: %s\n", pc.addr.Ip, pc.addr.Port, err)
			return
		}
		if n <= 0 {
			continue
		}
		msg, err := messages.DecodeMessageBinary(recvBuf[:n])
		if err != nil {
			util.Log.Printf("Error decoding message: %s\n", err)
			continue
		}
		select {
		case d.messageChan <- MessageChannelItem{Peer: pc.addr, Message: msg, Length: n}:
		case <-d.done:
			return
		}
	}
}

func (d *DataChannel) SendTo(msg messages.UDPMessage, remoteIp string, remotePort int) {
	if msg == nil || remoteIp == "" {
		return
	}
	d.mutex.Lock()
	pc, ok := d.peers[peerKey(remoteIp, remotePort)]
	d.mutex.Unlock()
	if !ok {
		util.Log.Printf("DataChannel has no channel registered for %s:%d\n", remoteIp, remotePort)
		return
	}
	select {
	case pc.sendChan <- msg:
	default:
		util.Log.Printf("DataChannel send queue full for %s:%d, dropping message\n", remoteIp, remotePort)
	}
}

// Read starts receiving on every registered channel and blocks until the
// connection is closed, mirroring Udp.Read.
func (d *DataChannel) Read(messageChan chan MessageChannelItem) {
	d.mutex.Lock()
	d.messageChan = messageChan
	d.reading = true
	for _, pc := range d.peers {
		go d.readLoop(pc)
	}
	d.mutex.Unlock()

	<-d.done
}

func (d *DataChannel) Close() {
	d.closeOnce.Do(func() {
		close(d.done)
		d.mutex.Lock()
		defer d.mutex.Unlock()
		for _, pc := range d.peers {
			if err := pc.rwc.Close(); err != nil {
				util.Log.Printf("DataChannel close error for %s:%d: %s\n", pc.addr.Ip, pc.addr.Port, err)
			}
		}
	})
}

func (d *DataChannel) IsInitialized() bool {
	return d != nil
}
