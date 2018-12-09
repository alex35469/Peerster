package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dedis/protobuf"
)

// Code retrieved from https://astaxie.gitbooks.io/build-web-application-with-golang/en/08.1.html
// Used to ckeck if an error occured
func checkError(err error, stop bool) {
	if err != nil {
		fmt.Println("Fatal error ", os.Stderr, err.Error())
		if stop {
			os.Exit(1)
		}

	}
}

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
	checkError(err, false)

	remoteGossiperAddr, err := net.ResolveUDPAddr("udp4", addr)
	checkError(err, false)

	_, err = myGossiper.conn.WriteTo(packetBytes, remoteGossiperAddr)
	if err != nil {
		fmt.Printf("Error: UDP write error: %v", err)
	}
}

// Pick one neighbor (But not the exception) we don't want to send back a rumor
// to the one that made us discovered it for exemple
func pickOneNeighbor(exception string) string {

	var s = rand.NewSource(time.Now().UnixNano())
	var R = rand.New(s)

	if len(myGossiper.neighbors) == 0 {
		return ""
	}

	if len(myGossiper.neighbors) == 1 {
		return myGossiper.neighbors[0]
	}

	picked := R.Intn(len(myGossiper.neighbors))
	for myGossiper.neighbors[picked] == exception {

		picked = R.Intn(len(myGossiper.neighbors))
	}

	return myGossiper.neighbors[picked]

}

func addNeighbor(neighbor string) {

	for _, n := range myGossiper.neighbors {
		if n == neighbor {
			return
		}
	}

	myGossiper.neighbors = append(myGossiper.neighbors, neighbor)

}

// CompareStatusPacket compare myVC and other VC's (Run in O(n) 2n if the two VC are shifted )
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
	i, j, toWithdraw := 0, 0, 0

	for i < maxMe && j < maxHim {

		// If the nextID field of the incomming packet is 1 we ignore it
		for otherVC.Want[j].NextID == 1 {
			j = j + 1
			toWithdraw++
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
			toWithdraw++
		}
		j = j + 1

	}

	if len(myVC.Want) < (len(otherVC.Want)-toWithdraw) || otherIsInadvance {
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

func (myVC *StatusPacket) SortVC() {

	// Since SliceSort use quick sort (best case O(n)) it's better to check if it's already sorted
	if sort.IsSorted(myVC) {
		return
	}

	sort.Sort(myVC)

}

func fireTicker() {

	ticker := time.NewTicker(ANTI_ENTROPY_DURATION)
	go func() {
		for range ticker.C {
			neighbor := pickOneNeighbor("")

			if neighbor != "" {
				sendTo(&GossipPacket{Status: myGossiper.myVC}, neighbor)
			}

		}

	}()
}

func fireRumor(rtimer int) {

	ticker := time.NewTicker(time.Duration(rtimer) * time.Second)

	go func() {
		for range ticker.C {
			// Same procedure (As if it is genereated by the client but there is no text in the content)
			routeMsg := SimpleMessage{
				Contents: "",
			}
			initiateRumorFromClient(&GossipPacket{Simple: &routeMsg})
		}

	}()

}

// NewGossiper Create the Gossiper
func NewGossiper(address, name, neighborsInit string) *Gossiper {
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	checkError(err, true)
	udpConn, err := net.ListenUDP("udp4", udpAddr)
	checkError(err, true)

	sp := StatusPacket{}
	sp.Want = make([]PeerStatus, 0)

	messagesHistoryInit := make(map[string]*SafeMsgsOrginHistory)
	messagesHistoryInit[name] = &SafeMsgsOrginHistory{}

	timersRecordInit := make(map[string][]*TimerForAck)
	safetimersRecord := SafeTimerRecord{timersRecord: timersRecordInit}

	routingTableInit := make(map[string]*RoutingTableEntry)
	FileRecordInit := make([]*FileRecord, 0)

	blockChannel := make(chan Block, 1)

	// Init the blockchain

	blockchain := Blockchain{
		blocks:           make(map[string]Block),
		nameHashMapping:  make(map[string]string),
		forksHead:        make([]Block, 0),
		forksHashMapping: make([]map[string]string, 0),
		orphansBlock:     make(map[string]Block),
	}

	nghInit := make([]string, 0)
	if neighborsInit != "" {
		nghInit = strings.Split(neighborsInit, ",")
	}

	sCtd := SafeChunkToDownload{
		tickers: make([]*time.Ticker, 0),
		chunks:  make([][]byte, 0),
		metas:   make([][]byte, 0), // Refer to the parent meta Hash
		dests:   make([]string, 0),
		fname:   make([]string, 0),
	}

	return &Gossiper{
		address:             udpAddr,
		conn:                udpConn,
		Name:                name,
		neighbors:           nghInit,
		myVC:                &sp,
		messagesHistory:     messagesHistoryInit,
		safeTimersRecord:    safetimersRecord,
		routingTable:        routingTableInit,
		safeFiles:           SafeFileRecords{files: FileRecordInit},
		safeCtd:             sCtd,
		blockChannel:        blockChannel,
		blockchain:          blockchain,
		pendingTransactions: pendingTransactions{transactions: make([]TxPublish, 0)},
	}
}

// From https://stackoverflow.com/questions/39868029/how-to-generate-a-sequence-of-numbers-in-golang?rq=1
func makeRange(min, max uint64) []uint64 {
	r := make([]uint64, max-min+1)
	for i := range r {
		r[i] = uint64(min + uint64(i))
	}
	return r
}

// retrieved on https://stackoverflow.com/questions/36000487/check-for-equality-on-slices-without-order
func sameStringSlice(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	// create a map of string -> int
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		// 0 value for int is 0, so just increment a counter for the string
		diff[_x]++
	}
	for _, _y := range y {
		// If the string _y is not in diff bail out early
		if _, ok := diff[_y]; !ok {
			return false
		}
		diff[_y]--
		if diff[_y] == 0 {
			delete(diff, _y)
		}
	}
	if len(diff) == 0 {
		return true
	}
	return false
}

func (t *TxPublish) Hash() (out [32]byte) {
	h := sha256.New()
	binary.Write(h, binary.LittleEndian,
		uint32(len(t.File.Name)))
	h.Write([]byte(t.File.Name))
	h.Write(t.File.MetafileHash)
	copy(out[:], h.Sum(nil))
	return
}

func (b *Block) Hash() (out [32]byte) {
	h := sha256.New()
	h.Write(b.PrevHash[:])
	h.Write(b.Nonce[:])
	binary.Write(h, binary.LittleEndian,
		uint32(len(b.Transactions)))
	for _, t := range b.Transactions {
		th := t.Hash()
		h.Write(th[:])
	}
	copy(out[:], h.Sum(nil))
	return
}

func (b *Block) Valid() bool {
	hash := b.Hash()
	return bytes.Equal(hash[:MINING_BYTES], make([]byte, MINING_BYTES, MINING_BYTES))

}
