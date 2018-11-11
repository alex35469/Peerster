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

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

// Special message only intended to the communication :
//  Client -------> Gossiper
type ClientMessage struct {
	File    string
	Request string
	Dest    string
}

type ClientPacket struct {
	Broadcast *SimpleMessage
	Private   *PrivateMessage
	CMessage  *ClientMessage
}

var UIPort, msg string
var dest, file, request string

// Init
func init() {
	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&msg, "msg", "", "message to be sent")
	flag.StringVar(&dest, "dest", "", "destination for the private message")
	flag.StringVar(&file, "file", "", "file to be indexed by the gossiper, or filename of the requested file")
	flag.StringVar(&request, "request", "", "request a chunk or metafile of this hash")
}

//########################## MAIN ######################

func main() {
	flag.Parse()
	packetToSend := ClientPacket{}

	if dest != "" && file != "" && request != "" {
		// The client is downloading a file

		// Telling the gossiper we want to download the file -file.
		packetToSend.CMessage = &ClientMessage{File: file, Request: request, Dest: dest}
		sendToGossiper(&packetToSend)
		return

	}

	// It's an indexing
	if file != "" && request == "" {
		fmt.Println("indexing")
		packetToSend.CMessage = &ClientMessage{File: file, Request: "", Dest: "Hello"}

	}

	// Broadcast message
	if dest == "" && msg != "" {
		sM := &SimpleMessage{OriginalName: "Client", RelayPeerAddr: "Null", Contents: msg}
		packetToSend.Broadcast = sM

	}

	// Private Message
	if dest != "" && msg != "" {

		packetToSend.Private = &PrivateMessage{
			Text:        msg,
			Destination: dest,
		}
	}

	sendToGossiper(&packetToSend)
}

//########################## END MAIN ######################

// Send a packet to the Gossiper
func sendToGossiper(packetToSend *ClientPacket) {

	packetBytes, err := protobuf.Encode(packetToSend)
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
