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

		if otherVC := newPacket.Status; otherVC != nil {

			// perform the sort just after reveiving the packet (Cannot rely on others in a decentralized system ;))
			otherVC.SortVC()
			myGossiper.myVC.Compare

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

func (myVC *StatusPacket) SortVC() {

	// Since SliceSort use quick sort (best case O(n)) it's better to check if it's already sorted
	if sort.IsSorted(myVC) {
		return
	}

	sort.Sort(myVC)

}

// Function to compare myVC and other VC's (Run in O(n) 2n if the two VC are shifted )
// Takes to SORTED VC and compare them
// To make no ambiguity (Both VC have msg that other doesn't have), we try to send the rumor first
// return : (case , identifier, nextID)
// Case +1 means we are in advance => todo : send a rumor message that the otherone does not have (using id & nextID)
// Case -1 means we are late => todo: send myVC to the neighbors (only if the other VC has every msgs we have)
// Case  0 means uptodate
func (myVC *StatusPacket) CompareStatusPacket(otherVC *StatusPacket) (int, string, uint32) {
	// If otherVC is emty, just fetch the first elem in myVC

	if len(otherVC.Want) == 0 && len(myVC.Want) != 0 {
		return 1, myVC.Want[0].Identifier, 0
	}

	if len(otherVC.Want) == 0 && len(myVC.Want) == 0 {
		return 0, "", 0
	}

	if len(otherVC.Want) != 0 && len(myVC.Want) == 0 {
		return -1, "", 0
	}

	otherIsInadvance := false

	var maxMe, maxHim int = len(myVC.Want), len(otherVC.Want)
	i, j := 0, 0

	for i < maxMe && j < maxHim {

		if myVC.Want[i].Identifier == otherVC.Want[j].Identifier {
			if myVC.Want[i].NextID > otherVC.Want[j].NextID {
				return 1, otherVC.Want[j].Identifier, otherVC.Want[j].NextID

			}
			if myVC.Want[i].NextID < otherVC.Want[j].NextID {
				otherIsInadvance = true
			}
			i = i + 1
			j = j + 1
			continue
		}

		if myVC.Want[i].Identifier < otherVC.Want[j].Identifier {

			return 1, myVC.Want[i].Identifier, 0
		}

		if myVC.Want[i].Identifier > otherVC.Want[j].Identifier {
			j = j + 1
			if j == maxHim && i == maxMe {
				return 1, myVC.Want[i].Identifier, 0
			}
		}
	}

	// We couldn't go to the end of myVC, thus otherVC could be scanned entirely
	// meaning: All the entries in myVC after i are not in otherVC
	if i < maxMe {
		return 1, myVC.Want[len(myVC.Want)-1].Identifier, 0

	}

	if len(myVC.Want) < len(otherVC.Want) || otherIsInadvance {
		return -1, "", 0
	}

	// The two lists are the same
	return 0, "", 0
}

// Making Satus message implementing interface sort
func (VC StatusPacket) Len() int           { return len(VC.Want) }
func (VC StatusPacket) Less(i, j int) bool { return VC.Want[i].Identifier < VC.Want[j].Identifier }
func (VC StatusPacket) Swap(i, j int)      { VC.Want[i], VC.Want[j] = VC.Want[j], VC.Want[i] }
