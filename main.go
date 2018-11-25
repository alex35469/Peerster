// Gossiper Main file. Accept the following arguments:
//  - UIPort  : port for the UI client (default "8080")
//  - gossipAddr  : ip:port for the gossiper (default "127.0.0.1:5000")
//  - name : name of the gossiper
//  - peers : coma separated list of peers of the form ip:UIPort
//  - simple : run gossiper in simple broadcast modified
// VC = Vector Clock

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/dedis/protobuf"
)

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

	// ############################## NEW SIMPLE MSG ###################

	if packet := newPacket.Simple; packet != nil && simple {
		processSimpleMsgGossiper(newPacket)
	}

	// ############################## NEW SEARCH MSG #################
	if packet := newPacket.SearchReply; packet != nil {
		processSearchReply(packet)
	}

	if packet := newPacket.SearchRequest; packet != nil {
		processSearchRequest(packet, receivedFrom)
	}

	// ############################## NEW DATA MSG #################

	if packet := newPacket.DataReply; packet != nil {
		fmt.Println("$RECEIVING DATA REPLY")
		processDataReply(packet)
	}

	if packet := newPacket.DataRequest; packet != nil {
		processDataRequest(packet)
	}

	// ################################ NEW PRIVATE ####################

	if packet := newPacket.Private; packet != nil {
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

// ##### PROCESSING SEARCH REPLY

func processSearchReply(packet *SearchReply) {
	fmt.Println("RECEIVING SEARCH REPLY ", packet)
}

func processSearchRequest(packet *SearchRequest, link string) {

	fmt.Printf("$SEARCH REQUEST from %s keywords %s budget %d link %s\n", packet.Origin, strings.Join(packet.Keywords, ","), packet.Budget, link)

	// If the search come back to us simply ignore it.ANTI_ENTROPY_DURATION
	if packet.Origin == myGossiper.Name {
		fmt.Println("$Ignoring")
		return
	}

	myGossiper.safeSearchesSeen.mux.Lock()

	// Firstly see if it's a duplicated searchRequests
	keywords := strings.Join(packet.Keywords, "")
	for _, sR := range myGossiper.safeSearchesSeen.searchesSeen {
		if keywords == strings.Join(sR.Keywords, "") && packet.Origin == sR.Origin {
			// Duplicate Search request
			fmt.Println("$YESSSS! FOUND DUPLICATES!")
			myGossiper.safeSearchesSeen.mux.Unlock()
			return
		}
	}

	// TODO:
	// Maybe put the

	results := make([]*SearchResult, 0)

	for _, f := range myGossiper.safeFiles.files {
		for _, key := range packet.Keywords {

			if matchName(f.Name, key) {

				// ChunkMap is simply the list from 0 to
				metaHashByte, err := hex.DecodeString(f.MetaHash)
				checkError(err, true)

				r := &SearchResult{
					FileName:     f.Name,
					ChunkMap:     makeRange(1, f.NbChunk),
					MetafileHash: metaHashByte,
				}

				results = append(results, r)

				// We found a match we proceed directly the next file
				break
			}
		}
	}

	fmt.Println("$Results: ", results)

	markSearchRequest(packet)

	packet.Budget = packet.Budget - 1

	myGossiper.safeSearchesSeen.mux.Unlock()
	distributeSearchRequest(*packet)
	craftAndSendSearchReply(results, packet)

}

func autodestruct(packet *SearchRequest) {
	fmt.Println("$Before Deleting : Searches seen: ", myGossiper.safeSearchesSeen.searchesSeen)
	myGossiper.safeSearchesSeen.mux.Lock()
	for i, sseen := range myGossiper.safeSearchesSeen.searchesSeen {
		if sseen == packet {
			copy(myGossiper.safeSearchesSeen.searchesSeen[i:], myGossiper.safeSearchesSeen.searchesSeen[i+1:])
			myGossiper.safeSearchesSeen.searchesSeen[len(myGossiper.safeSearchesSeen.searchesSeen)-1] = nil // or the zero value of T
			myGossiper.safeSearchesSeen.searchesSeen = myGossiper.safeSearchesSeen.searchesSeen[:len(myGossiper.safeSearchesSeen.searchesSeen)-1]
		}
	}
	fmt.Println("$After Deleting : Searches seen: ", myGossiper.safeSearchesSeen.searchesSeen)

	myGossiper.safeSearchesSeen.mux.Unlock()

}

func markSearchRequest(packet *SearchRequest) {

	// Mark the searchRequest
	myGossiper.safeSearchesSeen.searchesSeen = append(myGossiper.safeSearchesSeen.searchesSeen, packet)

	// Set up the autoDestructor timer
	t := time.NewTimer(SEEN_SEARCH_REQUEST_TIMEOUT)
	go func() {
		<-t.C
		// notSupposeToSendTo field = "" Because if there is a time out occure
		// for a peer, we might want to retry to send the message back to him
		autodestruct(packet)
	}()
}

func distributeSearchRequest(packet SearchRequest) {
	b := int(packet.Budget)
	n := len(myGossiper.neighbors)

	if b < 1 {
		//No budget anymore
		return
	}

	// budget == nb neighboors
	if b == n {
		packet.Budget = 1
		// For now on we don't consider the node how
		sendToAllNeighbors(&GossipPacket{SearchRequest: &packet}, "")
	}

	perm := rand.Perm(n)
	// budget < nb neihboors
	if b < n {
		packet.Budget = 1
		for i := 0; i < b; i++ {
			sendTo(&GossipPacket{SearchRequest: &packet}, myGossiper.neighbors[perm[i]])
		}
	}

	if b > n {
		packet.Budget = uint64(b / n)
		for k, i := range perm {
			sendTo(&GossipPacket{SearchRequest: &packet}, myGossiper.neighbors[i])

			if k == (n - b%n - 1) {
				packet.Budget += 1
			}

		}

	}

}

func craftAndSendSearchReply(results []*SearchResult, packet *SearchRequest) {

	record := myGossiper.routingTable[packet.Origin]

	// no need to create a search reply if we have no result
	// Or we don't know where to send the result
	if len(results) != 0 || record == nil {
		return
	}

	link := record.link
	sr := &SearchReply{
		Origin:      myGossiper.Name,
		Destination: packet.Origin,
		HopLimit:    HOP_LIMIT,
		Results:     results,
	}

	sendTo(&GossipPacket{SearchReply: sr}, link)

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

		fmt.Println("FLIPPED COIN sending rumor to", addrNeighbor)

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

	// PROCESSING FILE INDEXING AND FILE DOWNLOAD
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
				fmt.Println("CLIENT BAD FILE REQUEST hash doesn't match sha256 hash")
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

			// Filling the SafeChunkToDownload
			//myGossiper.safeCtd.mux.Lock()

			// Sending it
			//sendTo(&GossipPacket{DataRequest: &DataRequest{Origin: myGossiper.Name, Destination: packet.Dest, HashValue: request, HopLimit: HOP_LIMIT}}, link)
			fmt.Printf("DOWNLOADING metafile of %s from %s\n", packet.File, packet.Dest)
			requestNextChunk(packet.File, request, request, packet.Dest, len(myGossiper.safeCtd.tickers))

		}

	}

	// PROCESSING FILE SEARCH
	if packet := newPacket.CSearch; packet != nil {
		fmt.Printf("We received a file Search request with : kw= %s en len(kw) = %d , and bdgt = %d\n", packet.Keywords, len(packet.Keywords), packet.Budget)
		processSearchMsgFromClient(packet)
	}

}

func processSearchMsgFromClient(packet *ClientSearch) {
	// 1 Need to setup ticker that whill increase the budget
	//
	// craft Searchpacket

	sr := &SearchRequest{
		Origin:   myGossiper.Name,
		Budget:   packet.Budget,
		Keywords: packet.Keywords,
	}

	myGossiper.safeOngoingSearch.mux.Lock()

	if duplicateClientSearch(sr) {
		infos = append(infos, InfoElem{Desc: "Duplicate Search", Event: "search"})
		fmt.Println("CLIENT DUPLICATE SEARCH")
		myGossiper.safeOngoingSearch.mux.Unlock()
		return
	}

	stringKw := strings.Join(packet.Keywords, ",")
	fmt.Printf("CLIENT SEARCH REQUEST keywords %s budget %d link\n", stringKw, packet.Budget)

	infos = append(infos, InfoElem{Event: "search", Desc: "Searching " + stringKw + " with budget " + string(packet.Budget)})

	ticker := time.NewTicker(SEARCH_CLIENT_TIMEOUT)
	myGossiper.safeOngoingSearch.tickers = append(myGossiper.safeOngoingSearch.tickers, ticker)
	myGossiper.safeOngoingSearch.searches = append(myGossiper.safeOngoingSearch.searches, sr)

	// Mark the Search even if it comes from client
	myGossiper.safeSearchesSeen.mux.Lock()
	markSearchRequest(sr)
	myGossiper.safeSearchesSeen.mux.Unlock()

	if packet.Budget == 0 {
		// Performing the increasing search request
		go func() {
			sr.Budget = 2
			for range ticker.C {

				// we can delete the Search
				if sr.Budget > MAX_BUDGET {
					ticker.Stop()
					//Not even delete here but put it in the

					return
				}
				fmt.Println("Sending search. Budget = ", sr.Budget)
				distributeSearchRequest(*sr)
				sr.Budget = sr.Budget * 2
			}
		}()
	} else {
		// Perfoming the simple seach request (No need for ticker but we keep it in the list)
		ticker.Stop()
		distributeSearchRequest(*sr)
	}

	myGossiper.safeOngoingSearch.mux.Unlock()
}

func duplicateClientSearch(packet *SearchRequest) bool {

	keywords := strings.Join(packet.Keywords, "")

	for _, sr := range myGossiper.safeOngoingSearch.searches {
		if strings.Join(sr.Keywords, "") == keywords {
			return true
		}
	}
	return false
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
