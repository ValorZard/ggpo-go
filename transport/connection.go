// Package transport defines the wire format and connection contract shared by every GGPO network backend.
// A concrete backend (see transport/udp and transport/webrtc) serializes the same Message packets and satisfies Connection
// the protocol layer talks only to these interfaces, so it is
// agnostic to whether packets travel over UDP sockets or WebRTC data channels.
package transport

// Connection is the transport a backend exposes to the protocol layer. Peers
// are addressed by (ip, port); a backend that has no real network addresses
// (e.g. WebRTC) assigns each peer a synthetic address and uses it consistently.
type Connection interface {
	// SendTo serializes msg and delivers it to the peer at remoteIp:remotePort.
	SendTo(msg Message, remoteIp string, remotePort int)
	// Read blocks, decoding inbound packets and pushing them onto messageChan
	// tagged with the sending peer's address. Run it in its own goroutine.
	Read(messageChan chan MessageChannelItem)
	Close()
}

// MessageHandler receives a decoded packet along with the address it came from.
type MessageHandler interface {
	HandleMessage(ipAddress string, port int, msg Message, len int)
}

// PeerAddress identifies a peer within a Connection.
type PeerAddress struct {
	Ip   string
	Port int
}

// MessageChannelItem is one decoded inbound packet and its origin, as delivered
// over the channel passed to Connection.Read.
type MessageChannelItem struct {
	Peer    PeerAddress
	Message Message
	Length  int
}

type Stats struct {
	BytesSent   int
	PacketsSent int
	KbpsSent    float64
}
