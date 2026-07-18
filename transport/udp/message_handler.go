package udp

type MessageHandler interface {
	HandleUDPMessage(ipAddress string, port int, msg UDPMessage, len int)
}
