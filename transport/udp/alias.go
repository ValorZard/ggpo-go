package udp

// The wire format and connection contract now live in package transport so they
// can be shared with other backends (e.g. transport/webrtc). These aliases keep
// the udp.* names the protocol layer and its tests were written against, so
// moving the definitions required no changes to any consumer.

// TODO: remove these alias and just use the types directly

import "github.com/ikemen-engine/ggpo/transport"

type (
	UDPMessage         = transport.Message
	UDPMessageType     = transport.MessageType
	UDPHeader          = transport.Header
	UdpConnectStatus   = transport.ConnectStatus
	Connection         = transport.Connection
	MessageHandler     = transport.MessageHandler
	MessageChannelItem = transport.MessageChannelItem
	PeerAddress        = transport.PeerAddress

	SyncRequestPacket   = transport.SyncRequestPacket
	SyncReplyPacket     = transport.SyncReplyPacket
	QualityReportPacket = transport.QualityReportPacket
	QualityReplyPacket  = transport.QualityReplyPacket
	InputPacket         = transport.InputPacket
	InputAckPacket      = transport.InputAckPacket
	KeepAlivePacket     = transport.KeepAlivePacket
)

const (
	InvalidMsg       = transport.InvalidMsg
	SyncRequestMsg   = transport.SyncRequestMsg
	SyncReplyMsg     = transport.SyncReplyMsg
	InputMsg         = transport.InputMsg
	QualityReportMsg = transport.QualityReportMsg
	QualityReplyMsg  = transport.QualityReplyMsg
	KeepAliveMsg     = transport.KeepAliveMsg
	InputAckMsg      = transport.InputAckMsg

	UDPMsgMaxPlayers  = transport.MsgMaxPlayers
	MaxCompressedBits = transport.MaxCompressedBits
)

func NewUDPMessage(t UDPMessageType) UDPMessage { return transport.NewMessage(t) }
func DecodeMessageBinary(buffer []byte) (UDPMessage, error) {
	return transport.DecodeMessageBinary(buffer)
}
func EncodeMessage(packet UDPMessage) ([]byte, error) { return transport.EncodeMessage(packet) }
func DecodeMessage(buffer []byte) (UDPMessage, error) { return transport.DecodeMessage(buffer) }

func GetPacketTypeFromBuffer(buffer []byte) (UDPMessageType, error) {
	return transport.GetPacketTypeFromBuffer(buffer)
}
