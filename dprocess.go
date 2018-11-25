package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"
)

// ############## GOSSIPER ##############
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
	myGossiper.safeCtd.mux.Lock()

	for i, c := range myGossiper.safeCtd.chunks {

		fmt.Println("i = ", i, " while len(chunks) = ", len(myGossiper.safeCtd.chunks), " and len(dests) = ", len(myGossiper.safeCtd.dests))

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
					myGossiper.safeCtd.mux.Unlock()
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

					fmt.Printf("RECONSTRUCTED file %s\n", fname)
					cleaningCtd(c, nil)
					storeFile(fname, nil, myGossiper, k)
					infos = append(infos, InfoElem{Fname: fname, Event: "download", Desc: "downloaded", Hash: ""})

					myGossiper.safeCtd.mux.Unlock()
					// We found the match, we cleaned everything we can leave
					return

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

	}
	myGossiper.safeCtd.mux.Unlock()
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

	for i, _ := range myGossiper.safeCtd.chunks {

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

	fmt.Printf("<---- After cleaning: dests = %s and hashes = %x\n", myGossiper.safeCtd.dests, myGossiper.safeCtd.chunks)

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
			fmt.Printf("SENDING DATA REQUEST TO %s for Chunk %x\n", dest, hashChunck)
			sendTo(&GossipPacket{DataRequest: dr}, neighbor.link)
		}

		for range ticker.C {

			neighbor = myGossiper.routingTable[dest]
			if neighbor != nil {
				fmt.Printf("SENDING DATA REQUEST TO %s for Chunk %x\n", dest, hashChunck)
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

// ############## CLIENT ##############
