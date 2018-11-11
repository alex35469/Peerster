// Gossiper Main file. Accept the following arguments:
//  - UIPort  : port for the UI client (default "8080")
//  - gossipAddr  : ip:port for the gossiper (default "127.0.0.1:5000")
//  - name : name of the gossiper
//  - peers : coma separated list of peers of the form ip:UIPort
//  - simple : run gossiper in simple broadcast modified
// VC = Vector Clock

package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dedis/protobuf"
)

// Set the time out to 1 second
const TIME_OUT time.Duration = time.Second
const ANTI_ENTROPY_DURATION time.Duration = time.Second
const FILE_DURATION time.Duration = 5 * time.Second

//###### PEERSTER MESSAGES TYPES   #######
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
	FileName      string
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

type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

type GossipPacket struct {
	Simple      *SimpleMessage
	Rumor       *RumorMessage
	Status      *StatusPacket
	Private     *PrivateMessage
	DataRequest *DataRequest
	DataReply   *DataReply
}

// COMUNICATION WITH CLIENT
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

// Struct used to fetch a rumor when
// Stopping
type TimerForAck struct {
	rumor RumorMessage
	timer *time.Timer
}

type SafeTimerRecord struct {
	timersRecord map[string][]*TimerForAck
	mux          sync.Mutex
}

// The list
type SafeMsgsOrginHistory struct {
	history []*RumorMessage
	mux     sync.Mutex
}

// The Gossiper struct.
// address : address of the Gossiper
// connexion : connexion through which the gossiper speaks and listen
// Name used to identify his own messages
// neighbors : Peers that are "Onlink" i.e. We know their IP address
// messagesHistory : Will eventually countain all the messages from all peers in the net.
//									All the records from different origin can be locked independently due to SafeMsgsOrginHistory
// safeTimersRecord : Records of the timers regarding a particular rumor packet Can be locked

type SafeChunkToDownload struct {
	tickers []*time.Ticker
	chunks  [][]byte
	metas   [][]byte // Refer to the parent meta Hash
	dests   []string
	fname   []string
	mux     sync.Mutex
}

type SafeFileRecords struct {
	files []*FileRecord
	mux   sync.Mutex
}

type Gossiper struct {
	address          *net.UDPAddr
	conn             *net.UDPConn
	Name             string
	neighbors        []string
	myVC             *StatusPacket
	safeTimersRecord SafeTimerRecord
	messagesHistory  map[string]*SafeMsgsOrginHistory
	routingTable     map[string]*RoutingTableEntry
	mux              sync.Mutex
	safeFiles        SafeFileRecords
	safeCtd          SafeChunkToDownload
}

type RoutingTableEntry struct {
	link     string
	freshest uint32
}

type StackElem struct {
	Msg    string
	Origin string
	Dest   string
	Mode   string
}

type InfoElem struct {
	Fname string
	Hash  string
	Event string
	Desc  string
}

const UDP_PACKET_SIZE = 10000
const HOP_LIMIT = 10

var myGossiper *Gossiper

var UIPort, gossipAddr, name, neighborsInit string
var simple bool
var rtimer int

var stack = make([]StackElem, 0)
var infos = make([]InfoElem, 0)

//######################################## INIT #####################################

// Fetching the flags from the CLI
func init() {

	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	flag.StringVar(&name, "name", "NoName", "name of the gossiper")
	flag.StringVar(&neighborsInit, "peers", "", "name of the gossiper")
	flag.BoolVar(&simple, "simple", false, "run gossiper in simple broadcast modified")
	flag.IntVar(&rtimer, "rtimer", 0, "route rumors sending period in seconds, 0 to disable sending of route rumors")

}

//######################################## MAIN #####################################

func main() {
	// Parsing flags
	flag.Parse()
	myGossiper = NewGossiper(gossipAddr, name, neighborsInit)

	if !simple {
		fireTicker()
	}

	if rtimer > 0 {
		// Quickly Send an routinge rumor to be be known
		routeMsg := SimpleMessage{
			Contents: "",
		}
		initiateRumorFromClient(&GossipPacket{Simple: &routeMsg})

		fireRumor(rtimer)

	}

	go listenToGUI()

	go listenToGossipers()

	listenToClient()

}

//######################################## END MAIN #####################################

//###############################  Webserver connexion in wserver.go ##################

//###############################  Gossiper connexion ##################

// Listen to the Gossiper
func listenToGossipers() {
	// Setup the listener for the gossiper Port
	for {
		newPacket, addr := fetchMessagesGossiper(myGossiper.conn)

		// Process packet in a goroutine
		proccessPacketAndSend(newPacket, addr.String())
	}
}

func proccessPacketAndSend(newPacket *GossipPacket, receivedFrom string) {

	addNeighbor(receivedFrom)

	// ############################## NEW DATA REQUEST

	if packet := newPacket.DataReply; packet != nil {

		fmt.Println("RECEIVING DATA REPLY")
		processDataReply(packet)

	}

	if packet := newPacket.DataRequest; packet != nil {
		processDataRequest(packet)
	}

	if packet := newPacket.Simple; packet != nil && simple {
		processSimpleMsgGossiper(newPacket)
	}

	// ################################ NEW PRIVATE ####################

	if packet := newPacket.Private; packet != nil {

		// We received a private Message
		processPrivateMsgGossiper(packet)

	}

	// ################################ NEW RUMOR ####################

	if packet := newPacket.Rumor; packet != nil {

		processRumorMsgGossiper(packet, receivedFrom)

	}

	// ############################# NEW STATUS ##############
	if otherVC := newPacket.Status; otherVC != nil {
		processStatusMsgGossiper(otherVC, receivedFrom)
	}

}

// ##### PROCESSING DATA REPLY

func processDataReply(packet *DataReply) {

	// Check if the packet is not intended to us
	if packet.Destination != myGossiper.Name {

		// We don't know where to forward the PrivateMessage
		if myGossiper.routingTable[packet.Destination] == nil {
			return
		}

		//Can we send it?
		if hl := packet.HopLimit - 1; hl > 0 {
			packet.HopLimit = hl
			sendTo(&GossipPacket{DataReply: packet}, myGossiper.routingTable[packet.Destination].link)
		}
		return
	}

	// The hash doesn't match
	if !EqualityCheckRecievedData(packet.HashValue, packet.Data) {
		return
	}

	// Now look if the hash is a requested
	//myGossiper.safeCtd.mux.Lock()

	// Be careful of the ticker
	//defer myGossiper.safeCtd.mux.Unlock()

	for i, c := range myGossiper.safeCtd.chunks {

		myGossiper.safeCtd.mux.Lock()

		if bytes.Equal(c, packet.HashValue) && myGossiper.safeCtd.dests[i] == packet.Origin {
			//We needed to :
			// 1) Stop the ticker for the corresponding  (and the others timers)
			// 2) update the file record
			// 3) write to file the chunk
			// 4) ask for the next chunk
			// 5) if no chunk is needed any more, download it to _Downloads
			fmt.Printf("We Found a match at %d for %s and we have %d\n", i, myGossiper.safeCtd.dests, len(myGossiper.safeCtd.tickers))
			myGossiper.safeCtd.tickers[i].Stop()

			fname := myGossiper.safeCtd.fname[i]
			// If it is the Meta file
			if bytes.Equal(myGossiper.safeCtd.metas[i], c) {

				if (len(packet.Data) % 32) != 0 {
					fmt.Println("Wrong data size skipping this data")
					//myGossiper.safeCtd.mux.Unlock()
					return
				}

				metaHash := hex.EncodeToString(c)
				mf := hex.EncodeToString(packet.Data)
				re := regexp.MustCompile(`[a-f0-9]{64}`)
				mf2 := re.FindAllString(mf, -1) // Separate string in 64 letters (32bytes)
				fr := &FileRecord{Name: fname, MetaHash: metaHash, MetaFile: mf2, NbChunk: 0}

				// Add the file record to the files
				myGossiper.safeFiles.files = append(myGossiper.safeFiles.files, fr)

				hashChunck, err := hex.DecodeString(mf2[0])
				checkError(err, true)

				// Setup the ticker for next chunk and send the packet
				fmt.Printf("DOWNLOADING %s chunk 1 from %s\n", fname, packet.Origin)

				requestNextChunk(fname, hashChunck, myGossiper.safeCtd.metas[i], packet.Origin, i)
				// We clean the old hashes: and update them to the current hash that we want
				cleaningCtd(c, hashChunck)

				// Maybe put an unlock herer

			} else {
				// The metafile is already here
				k, l := chunkSeek(c, myGossiper)

				if l == -1 {
					fmt.Printf("Meta File already received but inconcitency when matching in the File Record with k = %d and hash = %s\n", k, hex.EncodeToString(c))
					//os.Exit(1)

				}
				fmt.Println("Writing to file")
				WriteChunk(fname, packet.Data)
				myGossiper.safeFiles.files[k].NbChunk += 1

				// Add the new chunk in the to dwonload list

				// We finished to download the whole file
				if myGossiper.safeFiles.files[k].NbChunk == len(myGossiper.safeFiles.files[k].MetaFile) {
					// Remove the hashes
					cleaningCtd(c, nil)

					fmt.Printf("RECONSTRUCTED file %s\n", fname)
					storeFile(fname, nil, myGossiper, k)
					infos = append(infos, InfoElem{Fname: fname, Event: "download", Desc: "downloaded", Hash: ""})
					// Delete the entry

				} else {

					// Setup the ticker for next chunk and send the packet
					hexaChunk := myGossiper.safeFiles.files[k].MetaFile[myGossiper.safeFiles.files[k].NbChunk]
					hashChunck, _ := hex.DecodeString(hexaChunk)

					fmt.Printf("DOWNLOADING %s chunk %d from %s\n", fname, myGossiper.safeFiles.files[k].NbChunk+1, packet.Origin)
					requestNextChunk(fname, hashChunck, myGossiper.safeCtd.metas[i], packet.Origin, i)
					cleaningCtd(c, hashChunck)

				}
			}
		}
		myGossiper.safeCtd.mux.Unlock()
	}
}

// Upgrade the chunk request that we want to send
func cleaningCtd(oldHash []byte, newHash []byte) {

	supressed := 0

	/*
		newTickers :=  []*time.Ticker
		newChunks := [][]byte
		newMetas :=   [][]byte // Refer to the parent meta Hash
			newDests :=   []string
		newFname :=   []string
	*/

	for i := 0; i < len(myGossiper.safeCtd.chunks); i++ {

		if newHash != nil {
			if bytes.Equal(myGossiper.safeCtd.chunks[i], oldHash) {
				// Waiting for a new chunk (Already downloaded because of other peers)
				myGossiper.safeCtd.chunks[i] = newHash
				myGossiper.safeCtd.tickers[i].Stop()
				requestNextChunk(myGossiper.safeCtd.fname[i], newHash, myGossiper.safeCtd.metas[i], myGossiper.safeCtd.dests[i], i)

			}
		} else {

			j := i - supressed

			if bytes.Equal(myGossiper.safeCtd.chunks[j], oldHash) {

				// Deleteing elements

				myGossiper.safeCtd.chunks = append(myGossiper.safeCtd.chunks[:j], myGossiper.safeCtd.chunks[j+1:]...)
				myGossiper.safeCtd.dests = append(myGossiper.safeCtd.dests[:j], myGossiper.safeCtd.dests[j+1:]...)
				myGossiper.safeCtd.fname = append(myGossiper.safeCtd.fname[:j], myGossiper.safeCtd.fname[j+1:]...)
				myGossiper.safeCtd.metas = append(myGossiper.safeCtd.metas[:j], myGossiper.safeCtd.metas[j+1:]...)

				myGossiper.safeCtd.tickers[j].Stop()
				myGossiper.safeCtd.tickers = append(myGossiper.safeCtd.tickers[:j], myGossiper.safeCtd.tickers[j+1:]...)

				supressed++
			}
		}
	}

}

// Set the timer and request next chunk
func requestNextChunk(fname string, hashChunck []byte, meta []byte, dest string, i int) {

	ticker := time.NewTicker(FILE_DURATION)
	if len(myGossiper.safeCtd.tickers) == i {
		myGossiper.safeCtd.chunks = append(myGossiper.safeCtd.chunks, hashChunck)
		myGossiper.safeCtd.dests = append(myGossiper.safeCtd.dests, dest)
		myGossiper.safeCtd.fname = append(myGossiper.safeCtd.fname, fname)
		myGossiper.safeCtd.metas = append(myGossiper.safeCtd.metas, meta)
		myGossiper.safeCtd.tickers = append(myGossiper.safeCtd.tickers, ticker)
	} else {
		myGossiper.safeCtd.tickers[i] = ticker
		myGossiper.safeCtd.chunks[i] = hashChunck
		myGossiper.safeCtd.dests[i] = dest
		myGossiper.safeCtd.fname[i] = fname
		myGossiper.safeCtd.metas[i] = meta
	}

	go func() {
		dr := &DataRequest{HopLimit: HOP_LIMIT, HashValue: hashChunck, Origin: myGossiper.Name, Destination: dest}
		// Send before waiting the ticker for the first time
		neighbor := myGossiper.routingTable[dest]
		if neighbor != nil {
			fmt.Printf("SENDING DATA REQUEST TO %s for Chunk %s\n", dest, hashChunck)
			sendTo(&GossipPacket{DataRequest: dr}, neighbor.link)
		}

		for range ticker.C {

			neighbor = myGossiper.routingTable[dest]
			if neighbor != nil {
				fmt.Printf("SENDING DATA REQUEST TO %s for Chunk %s\n", dest, hashChunck)
				sendTo(&GossipPacket{DataRequest: dr}, neighbor.link)
			}
		}
	}()
	fmt.Println("Finished")
}

// ########## DATA REQUEST
func processDataRequest(packet *DataRequest) {

	myGossiper.safeFiles.mux.Lock()
	defer myGossiper.safeFiles.mux.Unlock()

	// Check if the packet is not intended to us
	if packet.Destination != myGossiper.Name {

		// We don't know where to forward the Packet
		if myGossiper.routingTable[packet.Destination] == nil {
			return
		}

		//Can we send it?
		if hl := packet.HopLimit - 1; hl > 0 {
			packet.HopLimit = hl
			sendTo(&GossipPacket{DataRequest: packet}, myGossiper.routingTable[packet.Destination].link)
		}
		return
	}

	i, j := chunkSeek(packet.HashValue, myGossiper)

	// We don't have the chunk
	if i == -1 || j >= myGossiper.safeFiles.files[i].NbChunk {
		return
	}

	// We have the metaFile bur not the chunk yet
	data, hashValue := getDataAndHash(i, j, myGossiper)

	// Create the reply
	dReply := &DataReply{Data: data, HashValue: hashValue, Origin: myGossiper.Name, Destination: packet.Origin, HopLimit: HOP_LIMIT}

	if myGossiper.routingTable[packet.Origin] == nil {
		return
	}

	link := myGossiper.routingTable[packet.Origin].link
	fmt.Println("Sending DataReply ")
	sendTo(&GossipPacket{DataReply: dReply}, link)

}

// ###### PROCESSING STATUS MESSAGE
func processStatusMsgGossiper(otherVC *StatusPacket, receivedFrom string) {
	// perform the sort just after receiving the packet (Cannot rely on others to send sorted VC in a decentralized system ;))
	otherVC.SortVC()

	fmt.Print("STATUS from ", receivedFrom)
	for i, _ := range otherVC.Want {
		fmt.Printf(" peer %s nextID %d", otherVC.Want[i].Identifier, otherVC.Want[i].NextID)
	}
	fmt.Println()
	fmt.Println("PEERS " + strings.Join(myGossiper.neighbors[:], ","))

	myGossiper.safeTimersRecord.mux.Lock()

	timerForAcks, ok := myGossiper.safeTimersRecord.timersRecord[receivedFrom]
	wasAnAck := false
	var rumor = &RumorMessage{}

	if ok {
		rumor, wasAnAck = stopCorrespondingTimerAndTargetedRumor(timerForAcks, otherVC, receivedFrom)
	}

	myGossiper.safeTimersRecord.mux.Unlock()

	outcome, identifier, nextID := myGossiper.myVC.CompareStatusPacket(otherVC)

	if outcome == 0 {
		fmt.Printf("IN SYNC WITH %v​", receivedFrom)
		fmt.Println()
		if wasAnAck {
			flipACoinAndSend(rumor, receivedFrom)
		}
	}

	if outcome == -1 {
		sendTo(&GossipPacket{Status: myGossiper.myVC}, receivedFrom)
	}

	if outcome == 1 {

		msg := myGossiper.messagesHistory[identifier].history[nextID-1]

		// Restart the Rumongering process by sending a message
		// the other peer doesn't have
		sendTo(&GossipPacket{Rumor: msg}, receivedFrom)
		setTimer(msg, receivedFrom)
	}
}

// ###### PROCESSING RUMOR MESSAGE

func processRumorMsgGossiper(packet *RumorMessage, receivedFrom string) {

	updateRoutingTable(packet, receivedFrom)

	myGossiper.mux.Lock()

	safeHist, knownRecord := myGossiper.messagesHistory[packet.Origin]

	if !knownRecord {
		// Not very efficient to lock on the whole gossiper struct, should have put a mutex inside the message history and just
		// lock it for this part
		//  i.e. myGossiper.messagesHistory.mux.Lock()
		myGossiper.messagesHistory[packet.Origin] = &SafeMsgsOrginHistory{}
	}

	// We lock on a specific origin so that packets from different origin can still go
	// through. Only to packet that has two be process from the same origin have to wait
	myGossiper.messagesHistory[packet.Origin].mux.Lock()
	myGossiper.mux.Unlock()
	defer myGossiper.messagesHistory[packet.Origin].mux.Unlock()

	// Ignor message if it is a old or too new one (i.e. if it is out of order)
	if (!knownRecord && packet.ID == 1) || (knownRecord && packet.ID == uint32(len(safeHist.history)+1)) {

		if packet.Text != "" {
			fmt.Printf("RUMOR origin %s from %s ID %d contents %s", packet.Origin, receivedFrom, packet.ID, packet.Text)
			fmt.Println()
			fmt.Println("PEERS " + strings.Join(myGossiper.neighbors[:], ","))
		}
		//fmt.Printf("We have : known record = %v,  msgs = %v and we received: %v", knownRecord, msgs, packet)
		// keep trace of that incoming packet
		updateRecord(packet, knownRecord, receivedFrom)
		//myGossiper.messagesHistory[packet.Origin].mux.Unlock()

		// Send The Satus Packet as an Ack to sender
		sendTo(&GossipPacket{Status: myGossiper.myVC}, receivedFrom)

		// Send Rumor packet ( Handle timer, flip coin etc..)
		// No sense to send the rumor back (which will append the peer is isolated i.e len(neigbors) = 1)

		if len(myGossiper.neighbors) > 1 {
			sendRumor(packet, receivedFrom)
		}
	}

}

// ###### PROCESSING PRIVATE MESSAGE

func processPrivateMsgGossiper(packet *PrivateMessage) {

	if packet.Destination == myGossiper.Name {
		fmt.Printf("PRIVATE origin %s hop-limit %d contents %s\n", packet.Origin, packet.HopLimit, packet.Text)

		// Fill the stack
		stack = append(stack, StackElem{Origin: packet.Origin, Msg: packet.Text, Dest: packet.Destination, Mode: "Private"})
		return
	}

	// We don't know where to forward the PrivateMessage
	if myGossiper.routingTable[packet.Destination] == nil {
		return
	}

	// We can send it
	if hl := packet.HopLimit - 1; hl > 0 {
		packet.HopLimit = hl
		sendTo(&GossipPacket{Private: packet}, myGossiper.routingTable[packet.Destination].link)
	}
}

func processSimpleMsgGossiper(newPacket *GossipPacket) {
	fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
		newPacket.Simple.OriginalName,
		newPacket.Simple.RelayPeerAddr,
		newPacket.Simple.Contents,
	)

	sendMsgFromGossiperSimpleMode(newPacket)
	fmt.Println("PEERS " + strings.Join(myGossiper.neighbors[:], ","))
}

// Find associated timer of a partcicular Ack
// Return the corresponding Rumor Message of the ACK and True of it VC was here because of an Ack
// Return an empty Rumor Message and False if the VC is not related to an ACK (Comes from independent ticker on the other peer's machine)
func stopCorrespondingTimerAndTargetedRumor(timersForAcks []*TimerForAck, packet *StatusPacket, addr string) (*RumorMessage, bool) {

	// This loop shouldn't take a lot of time
	// Since for one peer, we delete the timers
	for j, t := range timersForAcks {

		// Find where the ID of the message
		in, i := packet.seekInVC(t.rumor.Origin)

		if in && packet.Want[i].NextID == t.rumor.ID+1 {

			// Stop the timer and Delete from the array
			// If we can stop it, we have to consider it as an ack
			stopped := t.timer.Stop()
			//fmt.Println("Stopped it")
			timersForAcks = append(timersForAcks[:j], timersForAcks[j+1:]...)

			myGossiper.safeTimersRecord.timersRecord[addr] = timersForAcks

			return &t.rumor, stopped
		}
	}

	return &RumorMessage{}, false

}

// Sending a rumor (Mongering with addr about packet)
// In charge of picking the neighbor, not sending to the address
// Setting the timer
func sendRumor(packet *RumorMessage, addr string) {

	addrNeighbor := pickOneNeighbor(addr)

	if addrNeighbor == "" {
		return
	}

	if packet.Text != "" {
		fmt.Println("MONGERING with", addrNeighbor)
	}

	sendTo(&GossipPacket{Rumor: packet}, addrNeighbor)

	setTimer(packet, addrNeighbor)

}

// Start & store the timer of the peer we are sending message to
// Store the sending packet in the record (to have a mean for stopping it)
func setTimer(packet *RumorMessage, addrNeighbor string) {

	t := time.NewTimer(TIME_OUT)
	myGossiper.safeTimersRecord.mux.Lock()
	myGossiper.safeTimersRecord.timersRecord[addrNeighbor] = append(myGossiper.safeTimersRecord.timersRecord[addrNeighbor], &TimerForAck{timer: t, rumor: *packet})
	myGossiper.safeTimersRecord.mux.Unlock()

	go func() {
		<-t.C
		// notSupposeToSendTo field = "" Because if there is a time out occure
		// for a peer, we might want to retry to send the message back to him
		flipACoinAndSend(packet, "")
	}()

}

func updateRoutingTable(packet *RumorMessage, receivedFrom string) {

	// We don't care about our own routing origin
	if packet.Origin == myGossiper.Name {
		return
	}

	// Updating the routing table
	// Create a record if new
	if myGossiper.routingTable[packet.Origin] == nil {
		myGossiper.routingTable[packet.Origin] = &RoutingTableEntry{freshest: packet.ID, link: receivedFrom}
		fmt.Println("DSDV " + packet.Origin + " " + receivedFrom)
		return
	}

	// Skip if same records
	if myGossiper.routingTable[packet.Origin].link == receivedFrom && myGossiper.routingTable[packet.Origin].freshest < packet.ID {
		myGossiper.routingTable[packet.Origin].freshest = packet.ID
		return
	}

	// Update if the packet is fresher
	if packet.ID > myGossiper.routingTable[packet.Origin].freshest {
		myGossiper.routingTable[packet.Origin].freshest = packet.ID
		myGossiper.routingTable[packet.Origin].link = receivedFrom
		fmt.Println("DSDV " + packet.Origin + " " + receivedFrom)
	}
}

func updateRecord(packet *RumorMessage, knownRecord bool, receivedFrom string) {
	// If we end up here it means it's a packet we have never seen before

	ps := PeerStatus{Identifier: packet.Origin, NextID: packet.ID + 1}
	//fmt.Printf("Inside updateRecord, outside if : %v  And VC %v with len %v", myGossiper.messagesHistory, myGossiper.myVC.Want, len(myGossiper.myVC.Want))
	if len(myGossiper.myVC.Want) == 0 {
		myGossiper.myVC.Want = append(myGossiper.myVC.Want, ps)
		myGossiper.messagesHistory[packet.Origin] = &SafeMsgsOrginHistory{}
		myGossiper.messagesHistory[packet.Origin].history = append(myGossiper.messagesHistory[packet.Origin].history, packet)

		if packet.Text != "" {
			stack = append(stack, StackElem{Origin: packet.Origin, Msg: packet.Text, Dest: "", Mode: "All"})
		}

		//fmt.Printf("Inside updateRecord: %v  And VC %v", myGossiper.messagesHistory, myGossiper.myVC.Want)
		return

	}
	// Use binary search to find where is located the orgin in the VC array
	// Or where it should go (if it doesn't exist)

	_, i := myGossiper.myVC.seekInVC(packet.Origin)

	// We have to insert at the end of the list
	if i == len(myGossiper.myVC.Want) {
		myGossiper.myVC.Want = append(myGossiper.myVC.Want, ps)
	}

	if myGossiper.myVC.Want[i].Identifier != packet.Origin {
		// Little trick to insert an element at position i in an array
		myGossiper.myVC.Want = append(myGossiper.myVC.Want, ps)
		copy(myGossiper.myVC.Want[i+1:], myGossiper.myVC.Want[i:])
	}

	if packet.Text != "" {
		stack = append(stack, StackElem{Origin: packet.Origin, Msg: packet.Text, Dest: "", Mode: "All"})
	}

	myGossiper.myVC.Want[i] = ps

	myGossiper.messagesHistory[packet.Origin].history = append(myGossiper.messagesHistory[packet.Origin].history, packet)
}

// packet is the packet we are
// notSupposeToSendTo is the peer we dont want to send the message:
// 	Typically the peer from which we received the message from¨
// 	Or the peer which send us an ACK
func flipACoinAndSend(packet *RumorMessage, notSupposeToSendTo string) {
	// If there is no neighbors
	if len(myGossiper.neighbors) < 1 {
		return
	}
	var s = rand.NewSource(time.Now().UnixNano())
	var R = rand.New(s)

	if R.Int()%2 == 0 {
		addrNeighbor := pickOneNeighbor(notSupposeToSendTo)
		if packet.Text != "" {
			fmt.Println("FLIPPED COIN sending rumor to", addrNeighbor)
		}
		if addrNeighbor == "" {
			return
		}
		sendTo(&GossipPacket{Rumor: packet}, addrNeighbor)
		setTimer(packet, addrNeighbor)

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

//############################### UI Connexion ##########################

func listenToClient() {
	// Setup the listener for the client's UIPort
	UIAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+UIPort)
	checkError(err, true)
	UIConn, err := net.ListenUDP("udp4", UIAddr)
	checkError(err, true)

	defer UIConn.Close()

	// Listennig
	for {
		newPacket, _ := fetchMessagesClient(UIConn)

		processMsgFromClient(newPacket)
	}
}

func processMsgFromClient(newPacket *ClientPacket) {

	// PROCESSING SIMPLE
	if packet := newPacket.Broadcast; packet != nil {
		fmt.Println("CLIENT MESSAGE " + packet.Contents)

		if simple {
			sendMsgFromClientSimpleMode(&GossipPacket{Simple: packet})
		}

		if !simple {
			initiateRumorFromClient(&GossipPacket{Simple: packet})
		}

	}

	// PROCESSIN PRIVATE
	if packet := newPacket.Private; packet != nil {
		fmt.Println("CLIENT MESSAGE " + packet.Text + " DEST " + packet.Destination)
		processPrivateMsgFromClient(packet)

	}

	// PROCESSING FILE INDEXING
	if packet := newPacket.CMessage; packet != nil {
		if packet.Request == "" {
			fr, err := ScanFile(packet.File)

			if err != nil {
				fmt.Println(err.Error())
				return
			}

			addFileRecord(fr, myGossiper)

			//fmt.Println(myGossiper.safeFiles.files)
		}

		if packet.Dest != "" && packet.File != "" && packet.Request != "" {

			// First, check if the file is already downloaded
			request, err := hex.DecodeString(packet.Request)

			if err != nil || len(request) != 32 {
				fmt.Println("CLIENT: Bad Request")
				return
			}

			i, _ := chunkSeek(request, myGossiper)

			// If we found the corresponding metafile record and the downoad is complete
			// we can already send back what the client ask
			if i != -1 && myGossiper.safeFiles.files[i].NbChunk == len(myGossiper.safeFiles.files[i].MetaFile) {

				fmt.Println("FILE FOUND IN THE INDEX")
				storeFile(packet.File, request, myGossiper, -1)
				return
			}

			// We don't know where to route it
			if myGossiper.routingTable[packet.Dest] == nil {
				return
			}

			// We Craft the dataRequest and send it

			link := myGossiper.routingTable[packet.Dest].link
			fmt.Println("Sending to link : ", link)

			// Filling the SafeChunkToDownload
			//myGossiper.safeCtd.mux.Lock()

			// Sending it
			//sendTo(&GossipPacket{DataRequest: &DataRequest{Origin: myGossiper.Name, Destination: packet.Dest, HashValue: request, HopLimit: HOP_LIMIT}}, link)
			fmt.Printf("DOWNLOADING metafile of %s from %s\n", packet.File, packet.Dest)
			requestNextChunk(packet.File, request, request, packet.Dest, len(myGossiper.safeCtd.tickers))

		}

	}

	// PROCESSING FILE REQUEST

}

func processPrivateMsgFromClient(packetToSend *PrivateMessage) {

	// We drop the packet if we don't have a proper entry in the table
	if rtEntry := myGossiper.routingTable[packetToSend.Destination]; rtEntry != nil {
		packetToSend.HopLimit = HOP_LIMIT
		packetToSend.ID = 0
		packetToSend.Origin = myGossiper.Name
		sendTo(&GossipPacket{Private: packetToSend}, rtEntry.link)

		// Fill the stack
		stack = append(stack, StackElem{Origin: packetToSend.Origin, Msg: packetToSend.Text, Dest: packetToSend.Destination, Mode: "Private"})
	}
	// We can also pick one random neighbor in the hop that he has an entry for that dest

}

func initiateRumorFromClient(newPacket *GossipPacket) {

	msgList, knownRecord := myGossiper.messagesHistory[myGossiper.Name]

	msgList.mux.Lock()

	id := len(msgList.history) + 1

	packet := &RumorMessage{Origin: myGossiper.Name, ID: uint32(id), Text: newPacket.Simple.Contents}

	updateRecord(packet, knownRecord, myGossiper.address.String())
	msgList.mux.Unlock()

	sendRumor(packet, myGossiper.address.String())

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

//################## HELPER Functions (Might be call in all above section) ###################

// Fetch a message that has been sent through a particular connection
func fetchMessagesGossiper(udpConn *net.UDPConn) (*GossipPacket, *net.UDPAddr) {
	var newPacket GossipPacket
	buffer := make([]byte, UDP_PACKET_SIZE)

	n, addr, err := udpConn.ReadFromUDP(buffer)
	checkError(err, true)
	err = protobuf.Decode(buffer[0:n], &newPacket)
	checkError(err, true)

	return &newPacket, addr
}

// Fetch a message that has been sent through a particular connection
func fetchMessagesClient(udpConn *net.UDPConn) (*ClientPacket, *net.UDPAddr) {
	var newPacket ClientPacket
	buffer := make([]byte, UDP_PACKET_SIZE)

	n, addr, err := udpConn.ReadFromUDP(buffer)
	checkError(err, true)
	err = protobuf.Decode(buffer[0:n], &newPacket)
	checkError(err, true)

	return &newPacket, addr
}

// Send a packet to every peers known by the gossiper (except the relayer)
func sendToAllNeighbors(packet *GossipPacket, relayer string) {

	// Extracting the peers

	for _, neighbor := range myGossiper.neighbors {
		if neighbor != relayer {
			sendTo(packet, neighbor)
		}
	}
}

func sendTo(packet *GossipPacket, addr string) {
	packetBytes, err := protobuf.Encode(packet)
	checkError(err, true)

	remoteGossiperAddr, err := net.ResolveUDPAddr("udp4", addr)
	checkError(err, true)

	_, err = myGossiper.conn.WriteTo(packetBytes, remoteGossiperAddr)
	if err != nil {
		fmt.Printf("Error: UDP write error: %v", err)
	}
}
