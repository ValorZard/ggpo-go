package udp

type Connection interface {
	SendTo(msg UDPMessage, remoteIp string, remotePort int)
	Close()
	Read(messageChan chan MessageChannelItem)
}

type peerAddress struct {
	Ip   string
	Port int
}

type MessageChannelItem struct {
	Peer    peerAddress
	Message UDPMessage
	Length  int
}
