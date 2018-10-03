// Gossiper Main file. Accept the following arguments:
//  - UIPort  : port for the UI client (default "8080")
//  - gossipAddr  : ip:port for the gossiper (default "127.0.0.1:5000")
//  - name : name of the gossiper
//  - peers : coma separated list of peers of the form ip:UIPort
//  - simple : run gossiper in simple broadcast modified

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

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

type Gossiper struct {
	address *net.UDPAddr
	conn    *net.UDPConn
	Name    string
}

const UDP_PACKET_SIZE = 1024

var peers string

var myGossiper *Gossiper

var UIPort, gossipAddr, name, peersInit string
var simple bool

//######################################## INIT #####################################

// Fetching the flags from the CLI
func init() {

	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	flag.StringVar(&name, "name", "", "name of the gossiper")
	flag.StringVar(&peersInit, "peers", "", "name of the gossiper")
	flag.BoolVar(&simple, "simple", true, "run gossiper in simple broadcast modified")

}

//######################################## MAIN #####################################

func main() {
	flag.Parse()
	myGossiper = NewGossiper(gossipAddr, name)

	fmt.Println(UIPort)
	// Do a goroutine to listen to Client
	listenToClient()

	// Do a go routine to listen to other peers gossipers
}

//######################################## END MAIN #####################################

func NewGossiper(address, name string) *Gossiper {
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	checkError(err)
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	checkError(err)

	return &Gossiper{
		address: udpAddr,
		conn:    udpConn,
		Name:    name,
	}
}

// Code retrieved from https://astaxie.gitbooks.io/build-web-application-with-golang/en/08.1.html
func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", os.Stderr, err.Error())
		os.Exit(1)
	}
}

// When a message is received,
func sendToPeers(packet *GossipPacket, sender string) {

	peersList := strings.Split(peers, ",")
	for _, v := range peersList {
		if v != sender {
			packetBytes, err := protobuf.Encode(packet)
			checkError(err)

			udpAddr, err := net.ResolveUDPAddr("udp4", v)
			checkError(err)
			udpConn, err := net.DialUDP("udp4", myGossiper.address, udpAddr)
			checkError(err)

			_, err = udpConn.WriteToUDP(packetBytes, udpAddr)
			if err != nil {
				fmt.Printf("Error: UDP write error: %v", err)
				continue
			}

		}
	}
}

func listenToClient() {
	// Setup the listener for the client's UIPort
	udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+UIPort)
	checkError(err)
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	checkError(err)

	defer udpConn.Close()

	fmt.Println(udpAddr)
	// Listennig
	for {
		newPacket := fetchMessages(udpConn)

		fmt.Println(newPacket.Simple.Contents)

	}

}

func fetchMessages(udpConn *net.UDPConn) *GossipPacket {
	var newPacket GossipPacket
	buffer := make([]byte, UDP_PACKET_SIZE)

	n, _, err := udpConn.ReadFromUDP(buffer)
	checkError(err)
	err = protobuf.Decode(buffer[0:n], &newPacket)
	checkError(err)

	return &newPacket
}

// When receiving a msg from another gossiper
func addPeer(peer string) {
	alreadyThere := strings.Contains(peers, peer)

	if alreadyThere {
		peers += peer
	}

}
