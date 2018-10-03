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
	peers   string
}

const UDP_PACKET_SIZE = 1024

var myGossiper *Gossiper

var UIPort, gossipAddr, name, peersInit string
var simple bool

//######################################## INIT #####################################

// Fetching the flags from the CLI
func init() {

	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	flag.StringVar(&name, "name", "NoName", "name of the gossiper")
	flag.StringVar(&peersInit, "peers", "", "name of the gossiper")
	flag.BoolVar(&simple, "simple", true, "run gossiper in simple broadcast modified")

}

//######################################## MAIN #####################################

func main() {
	// Parsing flags
	flag.Parse()
	myGossiper = NewGossiper(gossipAddr, name, peersInit)

	// Do a goroutine to listen to Client
	go listenToClient()

	listenToGossipers()
	// Do a go routine to listen to other peers gossipers
}

//######################################## END MAIN #####################################

func NewGossiper(address, name, peersInit string) *Gossiper {
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	checkError(err)
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	checkError(err)

	return &Gossiper{
		address: udpAddr,
		conn:    udpConn,
		Name:    name,
		peers:   peersInit,
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
func sendToPeers(packet *GossipPacket, relayer string) {

	peersList := strings.Split(myGossiper.peers, ",")
	for _, v := range peersList {
		if v != relayer {
			packetBytes, err := protobuf.Encode(packet)
			checkError(err)

			remoteGossiperAddr, err := net.ResolveUDPAddr("udp4", v)
			checkError(err)

			// Would be better to use the real myGossiper.address instead of nil but already used error
			remoteGossiperConn, err := net.DialUDP("udp4", nil, remoteGossiperAddr)
			checkError(err)

			_, err = remoteGossiperConn.Write(packetBytes)
			if err != nil {
				fmt.Printf("Error: UDP write error: %v", err)
				continue
			}
			remoteGossiperConn.Close()

		}
	}
}

func listenToGossipers() {
	// Setup the listener for the client's UIPort
	for {
		newPacket := fetchMessages(myGossiper.conn)

		fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
			newPacket.Simple.OriginalName,
			newPacket.Simple.RelayPeerAddr,

			newPacket.Simple.Contents,
		)
		sendMsgFromGossiper(newPacket)
		fmt.Println("PEERS " + myGossiper.peers)

	}
}

func sendMsgFromGossiper(packetToSend *GossipPacket) {
	// Add the relayer to the peers'field
	relayer := packetToSend.Simple.RelayPeerAddr
	addPeer(relayer)
	packetToSend.Simple.RelayPeerAddr = myGossiper.address.String()
	sendToPeers(packetToSend, relayer)

}

func listenToClient() {
	// Setup the listener for the client's UIPort
	UIAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+UIPort)
	checkError(err)
	UIConn, err := net.ListenUDP("udp4", UIAddr)
	checkError(err)

	defer UIConn.Close()

	// Listennig
	for {
		newPacket := fetchMessages(UIConn)

		fmt.Println("CLIENT MESSAGE " + newPacket.Simple.Contents)

		// Do maybe a go routine here or maybe not because of concurency
		sendMsgFromClient(newPacket)

		//sendToPeers(newPacket, sender)
	}
}

func sendMsgFromClient(packetToSend *GossipPacket) {
	packetToSend.Simple.OriginalName = myGossiper.Name
	packetToSend.Simple.RelayPeerAddr = myGossiper.address.String()
	// it's comming from the client so the send field should have none effects
	sendToPeers(packetToSend, "client")

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
	alreadyThere := strings.Contains(myGossiper.peers, peer)

	if !alreadyThere {
		myGossiper.peers += "," + peer
	}

}
