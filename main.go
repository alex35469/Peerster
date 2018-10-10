// Gossiper Main file. Accept the following arguments:
//  - UIPort  : port for the UI client (default "8080")
//  - gossipAddr  : ip:port for the gossiper (default "127.0.0.1:5000")
//  - name : name of the gossiper
//  - peers : coma separated list of peers of the form ip:UIPort
//  - simple : run gossiper in simple broadcast modified
// VC = Vector Clock

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dedis/protobuf"
)

// Set the time out to 1 second
var TIME_OUT time.Duration = time.Second
var s = rand.NewSource(time.Now().UnixNano())
var R = rand.New(s)

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

type TimerForAck struct {
	rumor RumorMessage
	timer *time.Timer
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
	myVC            *StatusPacket
	timersRecord    map[string]TimerForAck
	messagesHistory map[string][]*RumorMessage
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

		// make a go routine here
		go proccessPacketAndSend(newPacket, addr)
	}
}

func proccessPacketAndSend(newPacket *GossipPacket, addr *net.UDPAddr) {

	addNeighbor(addr.String())

	if packet := newPacket.Simple; packet != nil && simple {
		fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
			newPacket.Simple.OriginalName,
			newPacket.Simple.RelayPeerAddr,
			newPacket.Simple.Contents,
		)

		sendMsgFromGossiperSimpleMode(newPacket)

	}

	if packet := newPacket.Rumor; packet != nil {
		fmt.Printf("RUMOR origin %s from %s ID %d contents %s", packet.Origin, addr.String(), packet.ID, packet.Text)

		msgs, knownRecord := myGossiper.messagesHistory[packet.Origin]

		if (!knownRecord && packet.ID == 1) || (knownRecord && packet.ID == uint32(len(msgs)+1)) {

			updateRecord(packet, knownRecord)

			// Send The Satus Packet as an Ack to sender
			sendTo(&GossipPacket{Status: myGossiper.myVC}, addr.String())

			// Send Rumor packet ( Handle timer, flip coin etc..)
			sendRumor(packet, addr.String())
		}
		// Maybe set timer here or just after sending (Maybe more sense)
	}

	// ################### NEW StatusPacket ##############
	if otherVC := newPacket.Status; otherVC != nil {
		// should stop the stocked timer in the field timersRecord
		fmt.Print("STATUS from ", addr.String())
		for i, _ := range otherVC.Want {
			fmt.Printf("peer %s nextID %d ", otherVC.Want[i].Identifier, otherVC.Want[i].NextID)
		}

		timerForAck, ok := myGossiper.timersRecord[addr.String()]
		wasAnAck := false
		if ok {
			wasAnAck = timerForAck.timer.Stop()
		}
		// perform the sort just after reveiving the packet (Cannot rely on others to send sorted VC in a decentralized system ;))
		otherVC.SortVC()
		outcome, identifier, nextID := myGossiper.myVC.CompareStatusPacket(otherVC)

		if outcome == 0 {
			fmt.Println("IN SYNC WITH ​", addr.String())
			if wasAnAck {
				flipACoinAndSend(&timerForAck.rumor, addr.String())
			}
		}

		if outcome == -1 {
			sendTo(&GossipPacket{Status: myGossiper.myVC}, addr.String())
		}

		if outcome == 1 {
			// -1 to addapt to Lists
			msg := myGossiper.messagesHistory[identifier][nextID-1]
			sendTo(&GossipPacket{Rumor: msg}, addr.String())
		}

	}

	fmt.Println("PEERS " + myGossiper.neighbors)

}

func sendRumor(packet *RumorMessage, addr string) {

	addrNeighbor := pickOneNeighbor(addr)

	fmt.Println("MONGERING with", addr)
	sendTo(&GossipPacket{Rumor: packet}, addrNeighbor)

	// Start the timer of the peer we are sending message to
	// Store it in the record in case of
	t := time.NewTimer(TIME_OUT)
	myGossiper.timersRecord[addrNeighbor] = TimerForAck{timer: t, rumor: *packet}
	go func() {
		<-t.C
		flipACoinAndSend(packet, addr)
	}()

}

func updateRecord(packet *RumorMessage, knownRecord bool) {
	ps := PeerStatus{Identifier: packet.Origin, NextID: packet.ID + 1}

	// Use binary search to find where is located the orgin in the VC array
	// Or where it should go (if it doesn't exist)
	i := sort.Search(len(myGossiper.myVC.Want), func(arg1 int) bool {
		return myGossiper.myVC.Want[arg1].Identifier >= packet.Origin
	})

	if !knownRecord {
		// Little trick to insert an element at position i in an array
		myGossiper.myVC.Want = append(myGossiper.myVC.Want, ps)
		copy(myGossiper.myVC.Want[i+1:], myGossiper.myVC.Want[i:])
	}

	myGossiper.myVC.Want[i] = ps

	myGossiper.messagesHistory[packet.Origin] = append(myGossiper.messagesHistory[packet.Origin], packet)
}

func flipACoinAndSend(packet *RumorMessage, sender string) {
	if R.Int()%2 == 0 {
		addrNeighbor := pickOneNeighbor(sender)
		fmt.Println("FLIPPED COIN sending rumor to", addrNeighbor)
		sendTo(&GossipPacket{Rumor: packet}, addrNeighbor)
	}
}

// Send a message comming from another peer (not UI Client) port to all the peers
func sendMsgFromGossiperSimpleMode(packetToSend *GossipPacket) {
	// Add the relayer to the peers'field
	relayer := packetToSend.Simple.RelayPeerAddr
	addNeighbor(relayer)
	packetToSend.Simple.RelayPeerAddr = myGossiper.address.String()
	sendToAllNeighbors(packetToSend, relayer)

}

func addNeighbor(neighbors string) {
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
		if simple {
			sendMsgFromClientSimpleMode(newPacket)
		} else {
			msgList, knownRecord := myGossiper.messagesHistory[myGossiper.Name]
			id := len(msgList) + 1
			packet := &RumorMessage{Origin: myGossiper.Name, ID: uint32(id), Text: newPacket.Simple.Contents}

			updateRecord(packet, knownRecord)
			sendRumor(packet, myGossiper.address.String())

		}
		//sendToAllNeighbors(newPacket, sender)
	}
}

func sendMsgFromClientRumongering(packetToSend *GossipPacket) {

}

// Pick one neighbor (But not the exception) we don't want to send back a rumor
func pickOneNeighbor(exception string) string {

	neighborsList := strings.Split(myGossiper.neighbors, ",")

	picked := R.Intn(len(neighborsList))

	for neighborsList[picked] == exception {

		picked = R.Intn(len(neighborsList))
	}
	return neighborsList[picked]

}

// Send a message comming from the UIport to all the peers
func sendMsgFromClientSimpleMode(packetToSend *GossipPacket) {
	if packetToSend.Simple != nil {
		packetToSend.Simple.OriginalName = myGossiper.Name
		packetToSend.Simple.RelayPeerAddr = myGossiper.address.String()

		// it's comming from the client so the send field should have no effects
		sendToAllNeighbors(packetToSend, "client")
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
func sendToAllNeighbors(packet *GossipPacket, relayer string) {

	// Extracting the peers
	peersList := strings.Split(myGossiper.neighbors, ",")
	for _, neighbor := range peersList {
		if neighbor != relayer {
			sendTo(packet, neighbor)
		}
	}
}

func sendTo(packet *GossipPacket, addr string) {
	packetBytes, err := protobuf.Encode(packet)
	checkError(err)

	remoteGossiperAddr, err := net.ResolveUDPAddr("udp4", addr)
	checkError(err)

	_, err = myGossiper.conn.WriteTo(packetBytes, remoteGossiperAddr)
	if err != nil {
		fmt.Printf("Error: UDP write error: %v", err)
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

	sp := StatusPacket{}
	sp.Want = make([]PeerStatus, 1)

	messagesHistoryInit := make(map[string][]*RumorMessage)
	timersRecordInit := make(map[string]TimerForAck)

	// Init the gossiper's own record
	sp.Want[0] = PeerStatus{Identifier: name, NextID: 0}

	return &Gossiper{
		address:         udpAddr,
		conn:            udpConn,
		Name:            name,
		neighbors:       neighborsInit,
		myVC:            &sp,
		messagesHistory: messagesHistoryInit,
		timersRecord:    timersRecordInit,
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
// return : (outcome , identifier, nextID)
// outcome +1 means we are in advance => todo : send a rumor message that the otherone does not have (using id & nextID)
// outcome -1 means we are late => todo: send myVC to the neighbors (only if the other VC has every msgs we have)
// outcome  0 means uptodate
func (myVC *StatusPacket) CompareStatusPacket(otherVC *StatusPacket) (int, string, uint32) {
	// If otherVC is emty, just fetch the first elem in myVC

	if len(otherVC.Want) == 0 && len(myVC.Want) != 0 {
		return 1, myVC.Want[0].Identifier, 1
	}

	if len(otherVC.Want) == 0 && len(myVC.Want) == 0 {
		return 0, "", 0
	}

	if len(otherVC.Want) != 0 && len(myVC.Want) == 0 {
		return -1, "", 0
	}

	otherIsInadvance := false

	var maxMe, maxHim int = len(myVC.Want), len(otherVC.Want)
	i, j, to_with_draw := 0, 0, 0

	for i < maxMe && j < maxHim {

		// If the nextID field of the incomming packet is 1 we ignore it
		for otherVC.Want[j].NextID == 1 {
			j = j + 1
			to_with_draw = to_with_draw + 1
		}

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

			return 1, myVC.Want[i].Identifier, 1
		}

		if myVC.Want[i].Identifier > otherVC.Want[j].Identifier {
			j = j + 1
		}
	}

	// We couldn't go to the end of myVC, thus otherVC could be scanned entirely
	// meaning: All the entries in myVC after i are not in otherVC
	if i < maxMe {
		return 1, myVC.Want[i].Identifier, 1

	}

	// Seek for the remaining 1 fields in the rest of the other's VC
	for j < maxHim {
		if otherVC.Want[j].NextID == 1 {
			to_with_draw += 1
		}
		j = j + 1

	}

	if len(myVC.Want) < (len(otherVC.Want)-to_with_draw) || otherIsInadvance {
		return -1, "", 0
	}
	// The two lists are the same
	return 0, "", 0
}

// Making Satus message implementing interface sort
func (VC StatusPacket) Len() int           { return len(VC.Want) }
func (VC StatusPacket) Less(i, j int) bool { return VC.Want[i].Identifier < VC.Want[j].Identifier }
func (VC StatusPacket) Swap(i, j int)      { VC.Want[i], VC.Want[j] = VC.Want[j], VC.Want[i] }
