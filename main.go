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
	"sort"
	"strings"

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

type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

// The Gossiper struct.
// address : address of the Gossiper
// connexion : connexion through which the gossiper speaks and listen
// Name used to identify his own messages
// neighbors : Peers that are "Onlink" i.e. We know their IP address
// myVC : Vector clock of all known peers
// messagesHistory : Maybe have a dict with all messages from all known peers we don't earse any... in case one peer joins and have no way to recover all the messages

type Gossiper struct {
	address         *net.UDPAddr
	conn            *net.UDPConn
	Name            string
	neighbors       string
	myVC            *[]PeerStatus
	messagesHistory map[string][]string
}

const UDP_PACKET_SIZE = 1024

var myGossiper *Gossiper

var UIPort, gossipAddr, name, neighborsInit string
var simple bool

//######################################## INIT #####################################

// Fetching the flags from the CLI
func init() {

	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	flag.StringVar(&name, "name", "NoName", "name of the gossiper")
	flag.StringVar(&neighborsInit, "peers", "", "name of the gossiper")
	flag.BoolVar(&simple, "simple", true, "run gossiper in simple broadcast modified")

}

//######################################## MAIN #####################################

func main() {
	// Parsing flags
	flag.Parse()
	myGossiper = NewGossiper(gossipAddr, name, neighborsInit)

	// Do a goroutine to listen to Client
	go listenToClient()

	listenToGossipers()

}

//######################################## END MAIN #####################################

//###############################  Gossiper connexion ##################

// Listen to the Gossiper
func listenToGossipers() {
	// Setup the listener for the client's UIPort
	for {
		newPacket, addr := fetchMessages(myGossiper.conn)

		if packet := newPacket.Simple; packet != nil {
			fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
				newPacket.Simple.OriginalName,
				newPacket.Simple.RelayPeerAddr,
				newPacket.Simple.Contents,
			)
		}

		if packet := newPacket.Rumor; packet != nil {

		}

		if packet := newPacket.Status; packet != nil {

			vcOther := newPacket.Status.Want
			// perform the sort just after reveiving the packet (Cannot rely on others in a decentralized system ;))
			sort.Slice(vcOther, func(i, j int) bool {
				return vcOther[i].Identifier < vcOther[j].Identifier
			})
		}

		fmt.Println(addr.String())
		sendMsgFromGossiper(newPacket)
		fmt.Println("PEERS " + myGossiper.neighbors)

	}
}

// Send a message comming from another peer (not UI Client) port to all the peers
func sendMsgFromGossiper(packetToSend *GossipPacket) {
	// Add the relayer to the peers'field
	relayer := packetToSend.Simple.RelayPeerAddr
	addPeer(relayer)
	packetToSend.Simple.RelayPeerAddr = myGossiper.address.String()
	sendToPeers(packetToSend, relayer)

}

func addPeer(neighbors string) {
	alreadyThere := strings.Contains(myGossiper.neighbors, neighbors)

	if !alreadyThere {
		myGossiper.neighbors += "," + neighbors
	}
}

//############################### UI Connexion ##########################

func listenToClient() {
	// Setup the listener for the client's UIPort
	UIAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+UIPort)
	checkError(err)
	UIConn, err := net.ListenUDP("udp4", UIAddr)
	checkError(err)

	defer UIConn.Close()

	// Listennig
	for {
		newPacket, _ := fetchMessages(UIConn)

		fmt.Println("CLIENT MESSAGE " + newPacket.Simple.Contents)

		// Do maybe a go routine here or maybe not because of concurency
		sendMsgFromClient(newPacket)

		//sendToPeers(newPacket, sender)
	}
}

// Send a message comming from the UIport to all the peers
func sendMsgFromClient(packetToSend *GossipPacket) {

	if simple {
		packetToSend.Simple.OriginalName = myGossiper.Name
		packetToSend.Simple.RelayPeerAddr = myGossiper.address.String()

		// it's comming from the client so the send field should have no effects
		sendToPeers(packetToSend, "client")
	}

}

//############################### HELPER Functions (Called in both side) ######################

// Fetch a message that has been sent through a particular connection
func fetchMessages(udpConn *net.UDPConn) (*GossipPacket, *net.UDPAddr) {
	var newPacket GossipPacket
	buffer := make([]byte, UDP_PACKET_SIZE)

	n, addr, err := udpConn.ReadFromUDP(buffer)
	checkError(err)
	err = protobuf.Decode(buffer[0:n], &newPacket)
	checkError(err)
	fmt.Println(addr)

	return &newPacket, addr
}

// Send a packet to every peers known by the gossiper (except the relayer)
func sendToPeers(packet *GossipPacket, relayer string) {

	// Extracting the peers
	peersList := strings.Split(myGossiper.neighbors, ",")
	for _, v := range peersList {
		if v != relayer {
			packetBytes, err := protobuf.Encode(packet)
			checkError(err)

			remoteGossiperAddr, err := net.ResolveUDPAddr("udp4", v)
			checkError(err)

			_, err = myGossiper.conn.WriteTo(packetBytes, remoteGossiperAddr)
			if err != nil {
				fmt.Printf("Error: UDP write error: %v", err)
				continue
			}

		}
	}
}

// Code retrieved from https://astaxie.gitbooks.io/build-web-application-with-golang/en/08.1.html
// Used to ckeck if an error occured
func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", os.Stderr, err.Error())
		os.Exit(1)
	}
}

// Create the Gossiper
func NewGossiper(address, name, neighborsInit string) *Gossiper {
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	checkError(err)
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	checkError(err)

	initVC := make([]PeerStatus, 1)

	messagesHistoryInit := make(map[string][]string)

	//messagesHistoryInit[name] = append(messagesHistoryInit[name], "NewMsg" )

	// Init the gossiper's own record
	initVC[0] = PeerStatus{Identifier: name, NextID: 0}

	return &Gossiper{
		address:         udpAddr,
		conn:            udpConn,
		Name:            name,
		neighbors:       neighborsInit,
		myVC:            &initVC,
		messagesHistory: messagesHistoryInit,
	}
}

// Function to compare myVC and other VC's
// Takes to SORTED VC and compare them
// To make no ambiguity (Both VC have msg that other doesn't have), we try to send the rumor first
// return : (case , identifier, nextID)
// Case +1 means we are in advance => todo : send a rumor message that the otherone does not have (using id & nextID)
// Case -1 means we are late => todo: send myVC to the neighbors (only if the other VC has every msgs we have)
// Case  0 means uptodate
func (myVC *StatusPacket) CompareStatusPacket(otherVC *StatusPacket) (int, string, uint32) {

	/// NAYBE SIMPLE DO A DOUBLE LOOP it's  Simpler
	otherIsInadvance := false

	// If otherVC is emty, just fetch the first elem in myVC
	if len(otherVC.Want) == 0 && len(myVC.Want) != 0 {
		return 1, myVC.Want[0].Identifier, 0

	}

	var idx int = min(len(myVC.Want), len(otherVC.Want))

	for i := 0; i < idx; i++ {
		identifier := myVC.Want[i].Identifier
		nextID := myVC.Want[i].NextID

		if (identifier != otherVC.Want[i].Identifier) && (identifier < otherVC.Want[i].Identifier) {
			return 1, identifier, 0
		}

		if identifier > otherVC.Want[i].Identifier {
			otherIsInadvance = true
			continue
		}

		if nextID > otherVC.Want[i].NextID {
			return 1, identifier, otherVC.Want[i].NextID
		}

		if nextID < otherVC.Want[i].NextID {
			otherIsInadvance = true
		}

	}

	if len(myVC.Want) > len(otherVC.Want) {
		return 1, myVC.Want[len(myVC.Want)-1].Identifier, 0
	}

	// At this point, we know that we are at least not in advance

	if (len(myVC.Want) < len(otherVC.Want)) || otherIsInadvance {
		// The other one is in advance ... let him do all the computation
		return -1, "", 0
	}

	return 0, "", 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
