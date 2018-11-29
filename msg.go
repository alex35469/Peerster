package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// ###### PROCESSING STATUS MESSAGE
func processStatusMsgGossiper(otherVC *StatusPacket, receivedFrom string) {
	// perform the sort just after receiving the packet (Cannot rely on others to send sorted VC in a decentralized system ;))
	otherVC.SortVC()

	fmt.Print("STATUS from ", receivedFrom)
	for i := range otherVC.Want {
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

//######### CLIENT

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
