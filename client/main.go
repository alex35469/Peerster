// Client Main file. Accept the following arguments:
//  - UIPort  : port for the UI client (default "8080")
//  - msg : message to be sent

package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/dedis/protobuf"
)

// Peer simple message
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

type GossipPacket struct {
	Simple  *SimpleMessage
	Rumor   *RumorMessage
	Status  *StatusPacket
	Private *PrivateMessage
}

var UIPort, msg string
var dest string

// Init
func init() {
	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&msg, "msg", "", "message to be sent")
	flag.StringVar(&dest, "dest", "", "destination for the private message")
}

//########################## MAIN ######################

func main() {
	flag.Parse()
	packetToSend := GossipPacket{}

	if dest == "" {
		packetToSend = GossipPacket{Simple: &SimpleMessage{OriginalName: "Client", RelayPeerAddr: "Null", Contents: msg}}
	} else {

		packetToSend = GossipPacket{Private: &PrivateMessage{
			Text:        msg,
			Destination: dest,
		}}

	}

	sendToGossiper(packetToSend)
}

//########################## END MAIN ######################

// Send a packet to the Gossiper
func sendToGossiper(packetToSend GossipPacket) {

	packetBytes, err := protobuf.Encode(&packetToSend)
	checkError(err)

	udpAddr, err := net.ResolveUDPAddr("udp4", "localhost:"+UIPort)
	checkError(err)
	udpConn, err := net.DialUDP("udp4", nil, udpAddr)
	checkError(err)
	defer udpConn.Close()

	_, err = udpConn.Write(packetBytes)
	checkError(err)

}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", os.Stderr, err.Error())
		os.Exit(1)
	}
}
