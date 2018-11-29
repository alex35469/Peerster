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

	// Need to forward if not intended to us
	if packet.Destination != myGossiper.Name {
		// We don't know where to forward the PrivateMessage
		if myGossiper.routingTable[packet.Destination] == nil {
			return
		}

		// We can send it
		if hl := packet.HopLimit - 1; hl > 0 {
			packet.HopLimit = hl
			sendTo(&GossipPacket{SearchReply: packet}, myGossiper.routingTable[packet.Destination].link)
		}

		// Need to proceed
		fmt.Println("$RECEIVING SEARCH REPLY AND SENDING IT ", packet)
		return
	}

	myGossiper.safeSearchesSeen.mux.Lock()
	defer myGossiper.safeSearchesSeen.mux.Unlock()

	// Removing unmatching names or alreadyseen ones

	for _, result := range packet.Results {
		for dIndex, metaHash := range myGossiper.safeDownloadingFile.metaHash {

			// it matches an existing file
			if hex.EncodeToString(result.MetafileHash) == metaHash {

				updateDownloadingFile(result, dIndex, packet.Origin)

				// We pass to the next search result since it is supper
				break
			}

		}
		// It doesn't match any existing downloading file so we create the record
		createDownloadingFile(result, packet.Origin)

	}

	updateOngoingSearch()

	// Verify if with this search we have a match and if REQUIRED

}

// We then remove the ongoing search, and remove
// See if a Ongoing search is finished.
// Also if it is finished, we put the downloading
func updateOngoingSearch() {

	dSuppressed := 0
	oSuppressed := 0

	for dGlobal := range myGossiper.safeDownloadingFile.fname {
		dIndex := dGlobal - dSuppressed

		// We have to pass if the download is not finished yet
		if myGossiper.safeDownloadingFile.chunkCount[dIndex] != uint64(len(myGossiper.safeDownloadingFile.chunkMap[dIndex])) {
			fmt.Printf("$While cleaning, a Download was not finished name: %s\n  chunkcount= %d and len(chunkMap) = %d", myGossiper.safeDownloadingFile.fname[dIndex], myGossiper.safeDownloadingFile.chunkCount[dIndex], uint64(len(myGossiper.safeDownloadingFile.chunkMap[dIndex])))
			continue
		}

		// Seeking in the searches where we found a match
		for oGlobal := range myGossiper.safeOngoingSearch.searches {
			// Directly pass if we already seen that hash for that search
			oIndex := oGlobal - oSuppressed

			if alreadySeenThisHash(myGossiper.safeDownloadingFile.metaHash[dIndex], myGossiper.safeOngoingSearch.seenHashes[oIndex]) {
				continue
			}

			for _, k := range myGossiper.safeOngoingSearch.searches[oIndex].Keywords {

				if matchName(myGossiper.safeDownloadingFile.fname[dIndex], k) {
					// We have a match!!
					myGossiper.safeOngoingSearch.matches[oIndex]++
					myGossiper.safeOngoingSearch.seenHashes[oIndex] = append(myGossiper.safeOngoingSearch.seenHashes[oIndex], myGossiper.safeDownloadingFile.metaHash[dIndex])

					if myGossiper.safeOngoingSearch.matches[oIndex] >= REQUIRED_MATCH {
						// The Search is finished. Removing safeOngoing
						fmt.Println("SEARCH FINISHED")
						autodestruct(myGossiper.safeOngoingSearch.searches[oIndex], "ongoing")
						oSuppressed++
					}
					break
				}
			}
		}
		// We need to supprime dIndex since it's already downloaded
		// Moving the Downloading file to the ReadyToDownload Repo
		moveToReadyAndSuppress(dIndex)
		fmt.Println("$Did we really suppressed it?  Downloading", myGossiper.safeDownloadingFile)
		fmt.Println("$Did we really suppressed it?  ReadyToDownload", myGossiper.safeReadyToDownload)

		dSuppressed++
	}

}

func alreadySeenThisHash(thisHash string, alreadySeenHashes []string) bool {
	for _, h := range alreadySeenHashes {
		if h == thisHash {
			return true
		}
	}
	return false
}

func moveToReadyAndSuppress(dIndex int) {
	// Copying Part

	// See if the metahash already exist
	dMetaHash := myGossiper.safeDownloadingFile.metaHash[dIndex]
	moved := false
	for rIndex, rMetaHash := range myGossiper.safeReadyToDownload.metaHash {
		if rMetaHash == dMetaHash {
			// it already exist so we can simply update the infos here
			myGossiper.safeReadyToDownload.fname[rIndex] = myGossiper.safeDownloadingFile.fname[dIndex]
			myGossiper.safeReadyToDownload.chunkCount[rIndex] = myGossiper.safeDownloadingFile.chunkCount[dIndex]
			myGossiper.safeReadyToDownload.chunkMap[rIndex] = myGossiper.safeDownloadingFile.chunkMap[dIndex]
			myGossiper.safeReadyToDownload.metaHash[rIndex] = myGossiper.safeDownloadingFile.metaHash[dIndex]
			moved = true
		}
	}

	if !moved {
		// We have to create a new record for the file that has been downloaded
		myGossiper.safeReadyToDownload.fname = append(myGossiper.safeReadyToDownload.fname, myGossiper.safeDownloadingFile.fname[dIndex])
		myGossiper.safeReadyToDownload.chunkCount = append(myGossiper.safeReadyToDownload.chunkCount, myGossiper.safeDownloadingFile.chunkCount[dIndex])
		myGossiper.safeReadyToDownload.chunkMap = append(myGossiper.safeReadyToDownload.chunkMap, myGossiper.safeDownloadingFile.chunkMap[dIndex])
		myGossiper.safeReadyToDownload.metaHash = append(myGossiper.safeReadyToDownload.metaHash, myGossiper.safeDownloadingFile.metaHash[dIndex])
	}

	// Removing Part
	copy(myGossiper.safeDownloadingFile.fname[dIndex:], myGossiper.safeDownloadingFile.fname[dIndex+1:])
	myGossiper.safeDownloadingFile.fname[len(myGossiper.safeDownloadingFile.fname)-1] = "" // or the zero value of T
	myGossiper.safeDownloadingFile.fname = myGossiper.safeDownloadingFile.fname[:len(myGossiper.safeDownloadingFile.fname)-1]

	copy(myGossiper.safeDownloadingFile.chunkMap[dIndex:], myGossiper.safeDownloadingFile.chunkMap[dIndex+1:])
	myGossiper.safeDownloadingFile.chunkMap[len(myGossiper.safeDownloadingFile.chunkMap)-1] = nil // or the zero value of T
	myGossiper.safeDownloadingFile.chunkMap = myGossiper.safeDownloadingFile.chunkMap[:len(myGossiper.safeDownloadingFile.chunkMap)-1]

	copy(myGossiper.safeDownloadingFile.chunkCount[dIndex:], myGossiper.safeDownloadingFile.chunkCount[dIndex+1:])
	myGossiper.safeDownloadingFile.chunkCount[len(myGossiper.safeDownloadingFile.chunkCount)-1] = 0 // or the zero value of T
	myGossiper.safeDownloadingFile.chunkCount = myGossiper.safeDownloadingFile.chunkCount[:len(myGossiper.safeDownloadingFile.chunkCount)-1]

	copy(myGossiper.safeDownloadingFile.metaHash[dIndex:], myGossiper.safeDownloadingFile.metaHash[dIndex+1:])
	myGossiper.safeDownloadingFile.metaHash[len(myGossiper.safeDownloadingFile.metaHash)-1] = "" // or the zero value of T
	myGossiper.safeDownloadingFile.metaHash = myGossiper.safeDownloadingFile.metaHash[:len(myGossiper.safeDownloadingFile.metaHash)-1]
}

func createDownloadingFile(sr *SearchResult, origin string) {

	thisHash := hex.EncodeToString(sr.MetafileHash)
	everybodySeenThisBefore := true
	for _, alreadySeenHashes := range myGossiper.safeOngoingSearch.seenHashes {
		// We only create a Downloading file if we did not see it beforehand on the ongoing searches
		if !alreadySeenThisHash(thisHash, alreadySeenHashes) {
			everybodySeenThisBefore = false
			break
		}
	}

	if everybodySeenThisBefore {
		return
	}

	// We update all of the records	// We update all of the records
	myGossiper.safeDownloadingFile.fname = append(myGossiper.safeDownloadingFile.fname, sr.FileName)
	myGossiper.safeDownloadingFile.chunkCount = append(myGossiper.safeDownloadingFile.chunkCount, sr.ChunkCount)
	myGossiper.safeDownloadingFile.metaHash = append(myGossiper.safeDownloadingFile.metaHash, thisHash)

	chunkmap := make(map[uint64]string, len(sr.ChunkMap))
	for _, i := range sr.ChunkMap {
		chunkmap[i] = origin
	}

	// https://stackoverflow.com/questions/37532255/one-liner-to-transform-int-into-string/37533144
	chunks := strings.Trim(strings.Replace(fmt.Sprint(sr.ChunkMap), " ", ",", -1), "[]")

	fmt.Printf("FOUND match %s at %s metafile=%x ​chunks=%s\n", sr.FileName, origin, sr.MetafileHash, chunks)
	myGossiper.safeDownloadingFile.chunkMap = append(myGossiper.safeDownloadingFile.chunkMap, chunkmap)

}

func updateDownloadingFile(sr *SearchResult, dindex int, origin string) bool {
	updated := false
	for _, i := range sr.ChunkMap {
		_, ok := myGossiper.safeDownloadingFile.chunkMap[dindex][i]
		if !ok {
			myGossiper.safeDownloadingFile.chunkMap[dindex][i] = origin
			updated = true
		}

	}

	if updated {
		// https://stackoverflow.com/questions/37532255/one-liner-to-transform-int-into-string/37533144
		chunks := strings.Trim(strings.Replace(fmt.Sprint(sr.ChunkMap), " ", ",", -1), "[]")
		fmt.Printf("FOUND match %s at %s metafile=%x ​chunks=%s\n", sr.FileName, origin, sr.MetafileHash, chunks)
	}

	return uint64(len(myGossiper.safeDownloadingFile.chunkMap[dindex])) == myGossiper.safeDownloadingFile.chunkCount[dindex]

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
	for _, sR := range myGossiper.safeSearchesSeen.searchesSeen {
		if sameStringSlice(packet.Keywords, sR.Keywords) && packet.Origin == sR.Origin {
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
					ChunkCount:   uint64(len(f.MetaFile)),
				}
				fmt.Printf("len(MetaFile) = %d, NbChunk= %d", len(f.MetaFile), f.NbChunk)
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

func autodestruct(packet *SearchRequest, mode string) {

	if mode == "searchSeen" {

		fmt.Println("$Before Deleting : Searches seen: ", myGossiper.safeSearchesSeen.searchesSeen)
		for i, sseen := range myGossiper.safeSearchesSeen.searchesSeen {
			if sameStringSlice(sseen.Keywords, packet.Keywords) {
				copy(myGossiper.safeSearchesSeen.searchesSeen[i:], myGossiper.safeSearchesSeen.searchesSeen[i+1:])
				myGossiper.safeSearchesSeen.searchesSeen[len(myGossiper.safeSearchesSeen.searchesSeen)-1] = nil // or the zero value of T
				myGossiper.safeSearchesSeen.searchesSeen = myGossiper.safeSearchesSeen.searchesSeen[:len(myGossiper.safeSearchesSeen.searchesSeen)-1]
			}
		}
		fmt.Println("$After Deleting : Searches seen: ", myGossiper.safeSearchesSeen.searchesSeen)

	}
	if mode == "ongoing" {

		fmt.Println("$Before Deleting : Ongoing search: ", myGossiper.safeOngoingSearch.searches)

		for i, ongoing := range myGossiper.safeOngoingSearch.searches {
			if sameStringSlice(ongoing.Keywords, packet.Keywords) {

				// Stopping and deleting ticker
				myGossiper.safeOngoingSearch.tickers[i].Stop()
				copy(myGossiper.safeOngoingSearch.tickers[i:], myGossiper.safeOngoingSearch.tickers[i+1:])
				myGossiper.safeOngoingSearch.tickers[len(myGossiper.safeOngoingSearch.tickers)-1] = nil // or the zero value of T
				myGossiper.safeOngoingSearch.tickers = myGossiper.safeOngoingSearch.tickers[:len(myGossiper.safeOngoingSearch.tickers)-1]

				// Deleting on going searches
				copy(myGossiper.safeOngoingSearch.searches[i:], myGossiper.safeOngoingSearch.searches[i+1:])
				myGossiper.safeOngoingSearch.searches[len(myGossiper.safeOngoingSearch.searches)-1] = nil // or the zero value of T
				myGossiper.safeOngoingSearch.searches = myGossiper.safeOngoingSearch.searches[:len(myGossiper.safeOngoingSearch.searches)-1]

				// Deleting on going searches
				copy(myGossiper.safeOngoingSearch.matches[i:], myGossiper.safeOngoingSearch.matches[i+1:])
				myGossiper.safeOngoingSearch.matches[len(myGossiper.safeOngoingSearch.matches)-1] = 0 // or the zero value of T
				myGossiper.safeOngoingSearch.matches = myGossiper.safeOngoingSearch.matches[:len(myGossiper.safeOngoingSearch.matches)-1]

				// Deleting seenHashed
				copy(myGossiper.safeOngoingSearch.seenHashes[i:], myGossiper.safeOngoingSearch.seenHashes[i+1:])
				myGossiper.safeOngoingSearch.seenHashes[len(myGossiper.safeOngoingSearch.seenHashes)-1] = nil // or the zero value of T
				myGossiper.safeOngoingSearch.seenHashes = myGossiper.safeOngoingSearch.seenHashes[:len(myGossiper.safeOngoingSearch.seenHashes)-1]

			}
		}
		fmt.Println("$After Deleting : Ongoing search: ", myGossiper.safeOngoingSearch.searches)

	}

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
		myGossiper.safeSearchesSeen.mux.Lock()
		autodestruct(packet, "searchSeen")
		myGossiper.safeSearchesSeen.mux.Unlock()
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
				packet.Budget++
			}

		}

	}

}

func craftAndSendSearchReply(results []*SearchResult, packet *SearchRequest) {

	record := myGossiper.routingTable[packet.Origin]

	// no need to create a search reply if we have no result
	// Or we don't know where to send the result
	if len(results) == 0 || record == nil {
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
			if i != -1 && myGossiper.safeFiles.files[i].NbChunk == uint64(len(myGossiper.safeFiles.files[i].MetaFile)) {

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
		infos = append(infos, InfoElem{Desc: "Duplicate Search for matches or budget exceeded", Event: "search"})
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
	myGossiper.safeOngoingSearch.matches = append(myGossiper.safeOngoingSearch.matches, 0)
	myGossiper.safeOngoingSearch.seenHashes = append(myGossiper.safeOngoingSearch.seenHashes, make([]string, 0))

	if packet.Budget == 0 {
		// Performing the increasing search request
		go func() {
			sr.Budget = 2
			for range ticker.C {

				// we can delete the Search
				if sr.Budget > MAX_BUDGET {

					autodestruct(sr, "ongoing")
					//Not even delete here but put it in the

					return
				}
				fmt.Println("Sending search. Budget = ", sr.Budget)
				distributeSearchRequest(*sr)
				sr.Budget = sr.Budget * 2
			}
		}()
	} else {
		// Perfoming the simple seach request (Still with the timer)
		go func() {
			for range ticker.C {
				ticker.Stop()
				distributeSearchRequest(*sr)
				autodestruct(sr, "ongoing")
			}
		}()

	}

	myGossiper.safeOngoingSearch.mux.Unlock()
}

func duplicateClientSearch(packet *SearchRequest) bool {
	for _, sr := range myGossiper.safeOngoingSearch.searches {
		if sameStringSlice(packet.Keywords, sr.Keywords) {
			return true
		}
	}

	return false

}
