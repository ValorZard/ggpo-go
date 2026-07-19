package udp

import (
	"net"
	"strconv"

	"github.com/ikemen-engine/ggpo/internal/util"
	"github.com/ikemen-engine/ggpo/transport"
)

const (
	MaxUDPEndpoints  = 16
	MaxUDPPacketSize = 4096
)

type Udp struct {
	Stats transport.Stats // may not need this, may just be a service used by others

	socket         net.Conn
	messageHandler transport.MessageHandler
	listener       net.PacketConn
	localPort      int
	ipAddress      string
	sendChan       chan sendRequest
}

func getPeerAddress(address net.Addr) transport.PeerAddress {
	switch addr := address.(type) {
	case *net.UDPAddr:
		return transport.PeerAddress{
			Ip:   addr.IP.String(),
			Port: addr.Port,
		}
	}
	return transport.PeerAddress{}
}

func (u Udp) Close() {
	if u.listener != nil {
		u.listener.Close()
	}
}

type sendRequest struct {
	msg        transport.Message
	remoteIp   string
	remotePort int
}

func NewUdp(messageHandler transport.MessageHandler, localPort int) Udp {
	u := Udp{}

	u.sendChan = make(chan sendRequest, 256) // Create a buffered channel

	go func() { // Start a goroutine to handle sending of messages
		for req := range u.sendChan {
			RemoteEP := net.UDPAddr{IP: net.ParseIP(req.remoteIp), Port: req.remotePort}
			buf := req.msg.ToBytes()
			_, err := u.listener.WriteTo(buf, &RemoteEP)
			if err != nil {
				util.Log.Printf("WriteTo error: %s", err)
			}
		}
	}()

	u.messageHandler = messageHandler
	portStr := strconv.Itoa(localPort)

	u.localPort = localPort
	util.Log.Printf("binding udp socket to port %d.\n", localPort)
	u.listener, _ = net.ListenPacket("udp", "0.0.0.0:"+portStr)
	return u
}

// dst should be sockaddr
// maybe create Gob encoder and decoder members
// instead of creating them on each message send
func (u Udp) SendTo(msg transport.Message, remoteIp string, remotePort int) {
	if msg == nil || remoteIp == "" {
		return
	}

	// RemoteEP := net.UDPAddr{IP: net.ParseIP(remoteIp), Port: remotePort}
	// buf := msg.ToBytes()
	// u.listener.WriteTo(buf, &RemoteEP)
	u.sendChan <- sendRequest{msg: msg, remoteIp: remoteIp, remotePort: remotePort} // Add the request to the channel
}

func (u Udp) Read(messageChan chan transport.MessageChannelItem) {
	defer u.listener.Close()
	recvBuf := make([]byte, MaxUDPPacketSize*2)
	for {
		len, addr, err := u.listener.ReadFrom(recvBuf)
		if err != nil {
			util.Log.Printf("conn.Read error returned: %s\n", err)
			break
		} else if len <= 0 {
			util.Log.Printf("no data recieved\n")
		} else if len > 0 {
			util.Log.Printf("recvfrom returned (len:%d  from:%s).\n", len, addr.String())
			peer := getPeerAddress(addr)

			msg, err := transport.DecodeMessageBinary(recvBuf)
			if err != nil {
				util.Log.Printf("Error decoding message: %s", err)
				continue
			}
			messageChan <- transport.MessageChannelItem{Peer: peer, Message: msg, Length: len}
		}

	}
}

func (u Udp) IsInitialized() bool {
	return u.listener != nil
}
