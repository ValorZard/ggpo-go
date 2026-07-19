package mocks

import (
	"github.com/ikemen-engine/ggpo/internal/protocol"
	"github.com/ikemen-engine/ggpo/transport"
)

type FakeMessageHandler struct {
	Endpoint *protocol.UdpProtocol
}

func (f *FakeMessageHandler) HandleMessage(ipAddress string, port int, msg transport.Message, length int) {
	if f.Endpoint.HandlesMsg(ipAddress, port) {
		f.Endpoint.OnMsg(msg, length)
	}
}

func NewFakeMessageHandler(endpoint *protocol.UdpProtocol) FakeMessageHandler {
	f := FakeMessageHandler{}
	f.Endpoint = endpoint
	return f
}
