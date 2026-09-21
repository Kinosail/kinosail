package owneraccess

import (
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

type dnsTestPacket struct {
	data    []byte
	address net.Addr
}
type dnsTestConnection struct {
	net.PacketConn
	input   chan dnsTestPacket
	replies chan dnsTestPacket
}

func (connection *dnsTestConnection) ReadFrom(buffer []byte) (int, net.Addr, error) {
	packet, ok := <-connection.input
	if !ok {
		return 0, nil, io.EOF
	}
	return copy(buffer, packet.data), packet.address, nil
}

func (connection *dnsTestConnection) WriteTo(data []byte, address net.Addr) (int, error) {
	connection.replies <- dnsTestPacket{append([]byte(nil), data...), address}
	return len(data), nil
}

func TestPrivateDNSOnlyAnswersPairedDevices(t *testing.T) {
	t.Parallel()
	config, _, _ := ownerFixture(t)
	connection := &dnsTestConnection{input: make(chan dnsTestPacket, 3), replies: make(chan dnsTestPacket, 3)}
	current := &runtime{dns: connection}
	manager := &Manager{config: config, runtime: current, state: state{Enabled: true, Devices: []Device{{ProfileID: "owner", Revision: 1, Address: "10.92.0.2", PublicKey: "paired"}}}}
	message := dnsmessage.Message{Header: dnsmessage.Header{ID: 17}, Questions: []dnsmessage.Question{{Name: dnsmessage.MustNewName("server.example."), Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}}}
	query, err := message.Pack()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { manager.serveDNS(t.Context(), current, "server.example"); close(done) }()
	connection.input <- dnsTestPacket{query, &net.UDPAddr{IP: net.ParseIP("10.92.0.3"), Port: 4000}}
	connection.input <- dnsTestPacket{[]byte("invalid"), &net.UDPAddr{IP: net.ParseIP("10.92.0.2"), Port: 4000}}
	connection.input <- dnsTestPacket{query, &net.UDPAddr{IP: net.ParseIP("10.92.0.2"), Port: 4000}}
	close(connection.input)
	<-done
	select {
	case packet := <-connection.replies:
		assertPrivateDNSAnswer(t, packet)
	case <-time.After(time.Second):
		t.Fatal("paired DNS request received no answer")
	}
}

func assertPrivateDNSAnswer(t *testing.T, packet dnsTestPacket) {
	t.Helper()
	var response dnsmessage.Message
	if err := response.Unpack(packet.data); err != nil {
		t.Fatal(err)
	}
	if packet.address.String() != "10.92.0.2:4000" || response.ID != 17 || len(response.Answers) != 1 {
		t.Fatalf("unexpected DNS response=%#v destination=%s", response, packet.address)
	}
	answer, ok := response.Answers[0].Body.(*dnsmessage.AResource)
	if !ok || net.IP(answer.A[:]).String() != Address {
		t.Fatalf("answer=%#v", response.Answers)
	}
}
