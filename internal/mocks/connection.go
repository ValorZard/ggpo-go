package mocks

import (
	"fmt"
	"strconv"

	"github.com/ikemen-engine/ggpo/transport/udp"
)

type FakeConnection struct {
	SendMap         map[string][]udp.UDPMessage
	LastSentMessage udp.UDPMessage
}

func NewFakeConnection() FakeConnection {
	return FakeConnection{
		SendMap: make(map[string][]udp.UDPMessage),
	}
}
func (f *FakeConnection) SendTo(msg udp.UDPMessage, remoteIp string, remotePort int) {
	portStr := strconv.Itoa(remotePort)
	addresssStr := remoteIp + ":" + portStr
	sendSlice, ok := f.SendMap[addresssStr]
	if !ok {
		sendSlice := make([]udp.UDPMessage, 2)
		f.SendMap[addresssStr] = sendSlice
	}
	sendSlice = append(sendSlice, msg)
	f.SendMap[addresssStr] = sendSlice
	f.LastSentMessage = msg
}

func (f *FakeConnection) Read(messageChan chan udp.MessageChannelItem) {

}

func (f *FakeConnection) Close() {

}

type FakeP2PConnection struct {
	remoteHandler   udp.MessageHandler
	localPort       int
	remotePort      int
	localIP         string
	printOutput     bool
	LastSentMessage udp.UDPMessage
	MessageHistory  []udp.UDPMessage
}

func (f *FakeP2PConnection) SendTo(msg udp.UDPMessage, remoteIp string, remotePort int) {
	if f.printOutput {
		fmt.Printf("f.localIP %s f.localPort %d msg %s size %d\n", f.localIP, f.localPort, msg, msg.PacketSize())
	}
	f.LastSentMessage = msg
	f.MessageHistory = append(f.MessageHistory, msg)
	f.remoteHandler.HandleMessage(f.localIP, f.localPort, msg, msg.PacketSize())
}

func (f *FakeP2PConnection) Read(messageChan chan udp.MessageChannelItem) {
}

func (f *FakeP2PConnection) Close() {

}

func NewFakeP2PConnection(remoteHandler udp.MessageHandler, localPort int, localIP string) FakeP2PConnection {
	f := FakeP2PConnection{}
	f.remoteHandler = remoteHandler
	f.localPort = localPort
	f.localIP = localIP
	f.MessageHistory = make([]udp.UDPMessage, 10)
	return f
}

type FakeMultiplePeerConnection struct {
	remoteHandler []udp.MessageHandler
	localPort     int
	localIP       string
	printOutput   bool
}

func (f *FakeMultiplePeerConnection) SendTo(msg udp.UDPMessage, remoteIp string, remotePort int) {
	if f.printOutput {
		fmt.Printf("f.localIP %s f.localPort %d msg %s size %d\n", f.localIP, f.localPort, msg, msg.PacketSize())
	}
	for _, r := range f.remoteHandler {
		r.HandleMessage(f.localIP, f.localPort, msg, msg.PacketSize())
	}
}

func (f *FakeMultiplePeerConnection) Read(messageChan chan udp.MessageChannelItem) {
}

func (f *FakeMultiplePeerConnection) Close() {

}

func NewFakeMultiplePeerConnection(remoteHandler []udp.MessageHandler, localPort int, localIP string) FakeMultiplePeerConnection {
	f := FakeMultiplePeerConnection{}
	f.remoteHandler = remoteHandler
	f.localPort = localPort
	f.localIP = localIP
	return f
}
