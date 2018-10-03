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

type GossipPacket struct {
	Simple *SimpleMessage
}

var UIPort, msg string

// Init
func init() {
	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&msg, "msg", "", "message to be sent")
}

//########################## MAIN ######################

func main() {
	flag.Parse()
	simplemsg := SimpleMessage{OriginalName: "Client", RelayPeerAddr: "Null", Contents: msg}
	packetToSend := GossipPacket{&simplemsg}
	sendToGossiper(packetToSend)
}

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
