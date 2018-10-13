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
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/dedis/protobuf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Set the time out to 1 second
var TIME_OUT time.Duration = time.Second
var TICKER_DURATION time.Duration = time.Second

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

// The Gossiper struct.
// address : address of the Gossiper
// connexion : connexion through which the gossiper speaks and listen
// Name used to identify his own messages
// neighbors : Peers that are "Onlink" i.e. We know their IP address
// myVC : Vector clock of all known peers
// messagesHistory : Maybe have a dict with all messages from all known peers we don't earse any... in case one peer joins and have no way to recover all the messages

type Gossiper struct {
	address          *net.UDPAddr
	conn             *net.UDPConn
	Name             string
	neighbors        []string
	myVC             *StatusPacket
	safeTimersRecord SafeTimerRecord
	messagesHistory  map[string][]*RumorMessage
	mux              sync.Mutex
}

const UDP_PACKET_SIZE = 1024

var myGossiper *Gossiper

var UIPort, gossipAddr, name, neighborsInit string
var simple bool
var stack string

//######################################## INIT #####################################

// Fetching the flags from the CLI
func init() {

	flag.StringVar(&UIPort, "UIPort", "8080", "port for the UI client")
	flag.StringVar(&gossipAddr, "gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	flag.StringVar(&name, "name", "NoName", "name of the gossiper")
	flag.StringVar(&neighborsInit, "peers", "", "name of the gossiper")
	flag.BoolVar(&simple, "simple", false, "run gossiper in simple broadcast modified")

}

//######################################## MAIN #####################################

func main() {
	// Parsing flags
	flag.Parse()
	myGossiper = NewGossiper(gossipAddr, name, neighborsInit)

	fireTicker()

	go listenToGUI()

	go listenToGossipers()

	listenToClient()

}

//######################################## END MAIN #####################################

//###############################  Webserver connexion ##################
func sendID(w http.ResponseWriter, r *http.Request) {
	json := simplejson.New()
	json.Set("ID", myGossiper.Name)
	json.Set("addr", myGossiper.address.String())

	payload, err := json.MarshalJSON()

	checkError(err)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)

}

func msgsPost(w http.ResponseWriter, r *http.Request) {
	// from https://stackoverflow.com/questions/15672556/handling-json-post-request-in-go
	body, err := ioutil.ReadAll(r.Body)
	checkError(err)
	content := string(body)
	newPacket := SimpleMessage{Contents: content, OriginalName: myGossiper.Name, RelayPeerAddr: myGossiper.address.String()}
	processMsgFromClient(&GossipPacket{Simple: &newPacket})

	msgsGet(w, r)

}

func msgsGet(w http.ResponseWriter, r *http.Request) {

	json := simplejson.New()
	json.Set("msgs", stack)

	// flush the stack
	stack = ""

	payload, err := json.MarshalJSON()
	checkError(err)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}

func nodePost(w http.ResponseWriter, r *http.Request) {
	// from https://stackoverflow.com/questions/15672556/handling-json-post-request-in-go
	body, err := ioutil.ReadAll(r.Body)
	checkError(err)

	addNeighbor(string(body))

	nodeGet(w, r)

}

func nodeGet(w http.ResponseWriter, r *http.Request) {
	json := simplejson.New()
	json.Set("nodes", myGossiper.neighbors)

	payload, err := json.MarshalJSON()
	checkError(err)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}

func listenToGUI() {

	r := mux.NewRouter()
	r.HandleFunc("/id", sendID).Methods("GET")
	r.HandleFunc("/message", msgsPost).Methods("POST")
	r.HandleFunc("/message", msgsGet).Methods("GET")
	r.HandleFunc("/node", nodePost).Methods("POST")
	r.HandleFunc("/node", nodeGet).Methods("GET")

	http.Handle("/", r)

	http.ListenAndServe(":8080", handlers.CORS()(r))
}

//###############################  Gossiper connexion ##################

// Listen to the Gossiper
func listenToGossipers() {
	// Setup the listener for the gossiper Port
	for {
		newPacket, addr := fetchMessages(myGossiper.conn)

		// Process packet in a goroutine
		go proccessPacketAndSend(newPacket, addr.String())
	}
}

func proccessPacketAndSend(newPacket *GossipPacket, receivedFrom string) {

	addNeighbor(receivedFrom)

	if packet := newPacket.Simple; packet != nil && simple {
		fmt.Printf("SIMPLE MESSAGE origin %s from %s contents %s\n",
			newPacket.Simple.OriginalName,
			newPacket.Simple.RelayPeerAddr,
			newPacket.Simple.Contents,
		)

		sendMsgFromGossiperSimpleMode(newPacket)
		fmt.Println("PEERS " + strings.Join(myGossiper.neighbors[:], ","))
	}

	if packet := newPacket.Rumor; packet != nil {

		msgs, knownRecord := myGossiper.messagesHistory[packet.Origin]

		// Ignor message if it is a old or too new one (i.e. if it is out of order)
		if (!knownRecord && packet.ID == 1) || (knownRecord && packet.ID == uint32(len(msgs)+1)) {

			fmt.Printf("RUMOR origin %s from %s ID %d contents %s", packet.Origin, receivedFrom, packet.ID, packet.Text)
			fmt.Println()
			fmt.Println("PEERS " + strings.Join(myGossiper.neighbors[:], ","))

			//fmt.Printf("We have : known record = %v,  msgs = %v and we received: %v", knownRecord, msgs, packet)
			// keep trace of that incoming packet
			updateRecord(packet, knownRecord)

			// Send The Satus Packet as an Ack to sender
			sendTo(&GossipPacket{Status: myGossiper.myVC}, receivedFrom)

			// Send Rumor packet ( Handle timer, flip coin etc..)
			// No sense to send the rumor back (which will append the peer is isolated i.e len(neigbors) = 1)

			if len(myGossiper.neighbors) > 1 {
				sendRumor(packet, receivedFrom)
			}
		}

	}

	// ################### NEW StatusPacket ##############
	if otherVC := newPacket.Status; otherVC != nil {
		// perform the sort just after receiving the packet (Cannot rely on others to send sorted VC in a decentralized system ;))
		otherVC.SortVC()

		fmt.Print("STATUS from ", receivedFrom)
		for i, _ := range otherVC.Want {
			fmt.Printf(" peer %s nextID %d", otherVC.Want[i].Identifier, otherVC.Want[i].NextID)
		}
		fmt.Println()
		fmt.Println("PEERS " + strings.Join(myGossiper.neighbors[:], ","))

		timerForAcks, ok := myGossiper.safeTimersRecord.timersRecord[receivedFrom]
		wasAnAck := false
		var rumor = &RumorMessage{}

		if ok {
			rumor, wasAnAck = stopCorrespondingTimerAndTargetedRumor(timerForAcks, otherVC, receivedFrom)
		}

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
			// -1 to addapt to Lists
			msg := myGossiper.messagesHistory[identifier][nextID-1]

			// Restart the Rumongering process by sending a message
			// the other peer doesn't have
			//sendRumor(msg, receivedFrom)
			sendTo(&GossipPacket{Rumor: msg}, receivedFrom)
			// Do we have to set a timer here ???
			setTimer(msg, receivedFrom)
		}

	}

}

func fireTicker() {

	ticker := time.NewTicker(TICKER_DURATION)
	go func() {
		for range ticker.C {
			//fmt.Println("Fired")
			neighbor := pickOneNeighbor("")
			sendTo(&GossipPacket{Status: myGossiper.myVC}, neighbor)
		}

	}()
}

// Find associated timer of a partcicular Ack
// Return the corresponding Rumor Message of the ACK and True of it VC was here because of an Ack
// Return an empty Rumor Message and False if the VC is not related to an ACK (Comes from independent ticker on the other peer's machine)
func stopCorrespondingTimerAndTargetedRumor(timersForAcks []*TimerForAck, packet *StatusPacket, addr string) (*RumorMessage, bool) {
	//fmt.Println("Seeking for a timer to stop in ", timersForAcks)

	myGossiper.safeTimersRecord.mux.Lock()
	defer myGossiper.safeTimersRecord.mux.Unlock()

	// This loop shouldn't take a lot of time
	// Since for one peer, we delete the timers
	for j, t := range timersForAcks {

		// Find where the ID of the message
		in, i := packet.seekInVC(t.rumor.Origin)

		// Here
		if in && packet.Want[i].NextID == t.rumor.ID+1 {

			// Stop the timer and Delete from the array
			// If we can stop it, we have to consider it as an ack
			stopped := t.timer.Stop()
			//fmt.Println("Stopped it")
			timersForAcks = append(timersForAcks[:j], timersForAcks[j+1:]...)
			//println("Array after delatation: ", timersForAcks)
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

	fmt.Println("MONGERING with", addrNeighbor)
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
		//fmt.Println("Time Out")
		// notSupposeToSendTo field = "" Because if there is a time out occure
		// for a peer, we might want to retry to send the message back to him
		flipACoinAndSend(packet, "")
	}()

}

func updateRecord(packet *RumorMessage, knownRecord bool) {
	ps := PeerStatus{Identifier: packet.Origin, NextID: packet.ID + 1}
	//fmt.Printf("Inside updateRecord, outside if : %v  And VC %v with len %v", myGossiper.messagesHistory, myGossiper.myVC.Want, len(myGossiper.myVC.Want))
	if len(myGossiper.myVC.Want) == 0 {
		myGossiper.myVC.Want = append(myGossiper.myVC.Want, ps)
		myGossiper.messagesHistory[packet.Origin] = append(myGossiper.messagesHistory[packet.Origin], packet)

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

	stack = stack + packet.Origin + ":" + packet.Text + ","

	myGossiper.myVC.Want[i] = ps

	myGossiper.messagesHistory[packet.Origin] = append(myGossiper.messagesHistory[packet.Origin], packet)
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
	if R.Int()%2 == 0 {
		addrNeighbor := pickOneNeighbor(notSupposeToSendTo)
		fmt.Println("FLIPPED COIN sending rumor to", addrNeighbor)
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

func addNeighbor(neighbor string) {

	for _, n := range myGossiper.neighbors {
		if n == neighbor {
			return
		}
	}

	myGossiper.neighbors = append(myGossiper.neighbors, neighbor)

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

		go processMsgFromClient(newPacket)
		//sendToAllNeighbors(newPacket, sender)
	}
}

func processMsgFromClient(newPacket *GossipPacket) {

	fmt.Println("CLIENT MESSAGE " + newPacket.Simple.Contents)

	if simple {
		sendMsgFromClientSimpleMode(newPacket)
	} else {
		msgList, knownRecord := myGossiper.messagesHistory[myGossiper.Name]
		id := len(msgList) + 1
		packet := &RumorMessage{Origin: myGossiper.Name, ID: uint32(id), Text: newPacket.Simple.Contents}

		updateRecord(packet, knownRecord)
		sendRumor(packet, myGossiper.address.String())

	}
}

// Pick one neighbor (But not the exception) we don't want to send back a rumor
// to the one that made us discovered it
func pickOneNeighbor(exception string) string {

	if len(myGossiper.neighbors) == 1 {
		return myGossiper.neighbors[0]
	}

	picked := R.Intn(len(myGossiper.neighbors))
	for myGossiper.neighbors[picked] == exception {

		picked = R.Intn(len(myGossiper.neighbors))
	}

	return myGossiper.neighbors[picked]

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

	return &newPacket, addr
}

// Send a packet to every peers known by the gossiper (except the relayer)
func sendToAllNeighbors(packet *GossipPacket, relayer string) {

	// Extracting the peers

	for _, neighbor := range myGossiper.neighbors {
		fmt.Println("Neighbor : ", neighbor)
		fmt.Println("Relayer : ", relayer)
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
	sp.Want = make([]PeerStatus, 0)

	messagesHistoryInit := make(map[string][]*RumorMessage)
	timersRecordInit := make(map[string][]*TimerForAck)
	safetimersRecord := SafeTimerRecord{timersRecord: timersRecordInit}
	// Init the gossiper's own record

	return &Gossiper{
		address:          udpAddr,
		conn:             udpConn,
		Name:             name,
		neighbors:        strings.Split(neighborsInit, ","),
		myVC:             &sp,
		messagesHistory:  messagesHistoryInit,
		safeTimersRecord: safetimersRecord,
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

func (VC *StatusPacket) seekInVC(origin string) (bool, int) {

	if len(VC.Want) == 0 {
		return false, 0
	}

	i := sort.Search(len(VC.Want), func(arg1 int) bool {
		return VC.Want[arg1].Identifier >= origin
	})

	if len(VC.Want) == i {
		return false, i
	}

	if VC.Want[i].Identifier == origin {
		return true, i
	}
	return false, i

}
