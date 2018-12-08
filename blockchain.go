package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jinzhu/copier"
)

func processTxPublish(packet *TxPublish, recievedFrom string) {
	fmt.Println("We received a Tx published :) :", packet.File.Name)

	// We have to look in the pending transaction in case it is already there
	// We also have to look in the BlockChain (In the safeNameHashMapping)
	// If not we put the transaction in mapping so that the minner can mine it

	// We have to flood it as well

	valide := transfereInPendingTx(*packet)

	if !valide {
		fmt.Println("Not valide transaction. File with name : ", packet.File.Name)
		return
	}

	// Send the block To others if HopLimit != 0
	if hl := packet.HopLimit - 1; hl > 0 {
		packet.HopLimit = hl
		sendToAllNeighbors(&GossipPacket{TxPublish: packet}, recievedFrom)
	}

}

// transfereInPendingTx return true if it moves the file to pendingTransaction
// flase if the transaction is not valide
func transfereInPendingTx(transaction TxPublish) bool {

	myGossiper.blockchain.mux.Lock()
	defer myGossiper.blockchain.mux.Unlock()

	myGossiper.pendingTransactions.mux.Lock()
	defer myGossiper.pendingTransactions.mux.Unlock()

	if !nameOk(transaction.File.Name) {
		return false
	}

	myGossiper.pendingTransactions.transactions = append(myGossiper.pendingTransactions.transactions, transaction)
	return true
}

func processBlockPublish(packet *BlockPublish, recievedFrom string) {

	// Check if the incoming block is valid
	if !packet.Block.Valid() {
		return
	}

	fmt.Println("$RECEIVED BLOCKPUBLISH")

	// Send the block to the channel such that processBlock Can handle it
	myGossiper.blockChannel <- packet.Block

	broadcastBlock(packet, recievedFrom)

	// Make sure that the incoming block's file name
	// are not already in the blockchain

}

func broadcastBlock(packet *BlockPublish, recievedFrom string) {
	// Send the block To others if HopLimit != 0
	if hl := packet.HopLimit - 1; hl > 0 {
		packet.HopLimit = hl
		sendToAllNeighbors(&GossipPacket{BlockPublish: packet}, recievedFrom)
	}
}

func processBlock() {

BLOCKLOOP:
	for currentBlock := range myGossiper.blockChannel {

		h := currentBlock.Hash()
		ph := currentBlock.PrevHash
		currentHash := hex.EncodeToString(h[:])
		prevHash := hex.EncodeToString(ph[:])

		myGossiper.blockchain.mux.Lock()

		_, seenBlock := myGossiper.blockchain.blocks[currentHash]
		parentBlock, seenParent := myGossiper.blockchain.blocks[prevHash]

		// We add a block to the blockchain only if we've never seen that block before
		if !(!seenBlock && seenParent || bytes.Equal(ph[:], make([]byte, 32, 32))) {
			fmt.Printf("Warning: seenBlock = %t seenParent = %t\n", seenBlock, seenParent)
			myGossiper.blockchain.printChain()
			myGossiper.blockchain.mux.Unlock()
			continue
		}
		// Adding The Block to our blockchains
		myGossiper.blockchain.blocks[currentHash] = currentBlock

		// HANDLING INCOMING BLOCK THAT EXTEND LONGEST CHAIN
		// Work also for the first block since parent = nil head = nil
		if parentBlock.Hash() == myGossiper.blockchain.head.Hash() {
			// contribute to the current longest chain

			// Checking the transactions
			for _, tx := range currentBlock.Transactions {
				_, alreadySeen := myGossiper.blockchain.nameHashMapping[tx.File.Name]
				if alreadySeen {
					// The underlying transaction is already in the blockchain
					fmt.Println("Transaction in the chain has already been seen")
					myGossiper.blockchain.mux.Unlock()
					continue BLOCKLOOP
				}
			}

			// Filling the ledger & withdrowing pending
			toWithdraw := make([]TxPublish, 0)
			for _, tx := range currentBlock.Transactions {
				myGossiper.blockchain.nameHashMapping[tx.File.Name] = hex.EncodeToString(tx.File.MetafileHash)
				toWithdraw = append(toWithdraw, tx)
			}

			//fmt.Println("toWithDraw: ", toWithdraw)
			//fmt.Println("transaction in the current mined block : ", currentBlock.Transactions)

			withDrawFromPending(toWithdraw)

			copier.Copy(&myGossiper.blockchain.head, &currentBlock)

			myGossiper.blockchain.lengthLongestChain++
			myGossiper.blockchain.printChain()
			fmt.Println(myGossiper.blockchain.lengthLongestChain)
			myGossiper.blockchain.mux.Unlock()
			continue
		}

		// HANDLING INCOMING BLOCK THAT'S EXPAND THE CHAIN OF A FORK
		for i, fHead := range myGossiper.blockchain.forksHead {
			if parentBlock.Hash() == fHead.Hash() {

				// Checking the transactions
				for _, tx := range currentBlock.Transactions {
					_, alreadySeen := myGossiper.blockchain.forksHashMapping[i][tx.File.Name]
					if alreadySeen {

						// The underlying transaction is already in the blockchain
						fmt.Println("Transaction contained in the fork have already been seen. abording transaction")
						myGossiper.blockchain.mux.Unlock()
						continue BLOCKLOOP
					}
				}

				// Filling the ledger of the particular fork
				for _, tx := range currentBlock.Transactions {
					myGossiper.blockchain.forksHashMapping[i][tx.File.Name] = hex.EncodeToString(tx.File.MetafileHash)
				}

				// Adding the head of the fork
				myGossiper.blockchain.forksHead[i] = currentBlock
				myGossiper.blockchain.forksLength[i]++

				// Adding the trasaction to the relevant mapping

				if myGossiper.blockchain.forksLength[i] > myGossiper.blockchain.lengthLongestChain {
					rewindNumber := rewind(i)
					fmt.Printf("FORK-LONGER rewind %d blocks\n", rewindNumber)
				} else {

					fmt.Printf("FORK-SHORTER %s\n", currentHash)
				}

				myGossiper.blockchain.mux.Unlock()
				continue BLOCKLOOP
			}
		}

		// HANDLING THE CREATION OF A NEW FORK
		createNewFork(currentBlock)

		myGossiper.blockchain.mux.Unlock()
	}
}

func createNewFork(forkHead Block) {
	myGossiper.blockchain.forksHead = append(myGossiper.blockchain.forksHead, forkHead)

	forksHashMapping := make(map[string]string)
	for _, tx := range forkHead.Transactions {
		forksHashMapping[tx.File.Name] = hex.EncodeToString(tx.File.MetafileHash)
	}

	myGossiper.blockchain.forksHashMapping = append(myGossiper.blockchain.forksHashMapping, forksHashMapping)

	myGossiper.blockchain.forksLength = append(myGossiper.blockchain.forksLength, findLength(forkHead))
	fmt.Println("$Forkcreated")
	fmt.Printf("FORK-SHORTER %x\n", forkHead.Hash())

}

func findLength(head Block) int {

	length := 1
	fmt.Println()
	for {
		if bytes.Equal(head.PrevHash[:], make([]byte, 32, 32)) {
			return length
		}

		head = myGossiper.blockchain.blocks[hex.EncodeToString(head.PrevHash[:])]
		length++
	}

}

func FindRewindNumb(fork int) int {
	rewindNumber := 1
	head := myGossiper.blockchain.head
	forkHead := myGossiper.blockchain.forksHead[fork]

	fmt.Println("Entering to the rewind loop")

	for {
		for {

			if bytes.Equal(head.PrevHash[:], forkHead.PrevHash[:]) {
				fmt.Println("Found rewind")
				return rewindNumber
			}

			if bytes.Equal(head.PrevHash[:], make([]byte, 32, 32)) {
				head = myGossiper.blockchain.head
				break
			}

			head = myGossiper.blockchain.blocks[hex.EncodeToString(head.PrevHash[:])]

		}
		if bytes.Equal(forkHead.PrevHash[:], make([]byte, 32, 32)) {
			break
		}
		forkHead = myGossiper.blockchain.blocks[hex.EncodeToString(forkHead.PrevHash[:])]
		rewindNumber++
	}

	return rewindNumber
}

func rewind(fork int) int {
	//
	rewindNumb := FindRewindNumb(fork)

	// Swapping
	myGossiper.blockchain.head, myGossiper.blockchain.forksHead[fork] = myGossiper.blockchain.forksHead[fork], myGossiper.blockchain.head
	myGossiper.blockchain.lengthLongestChain, myGossiper.blockchain.forksLength[fork] = myGossiper.blockchain.forksLength[fork], myGossiper.blockchain.lengthLongestChain
	myGossiper.blockchain.nameHashMapping, myGossiper.blockchain.forksHashMapping[fork] = myGossiper.blockchain.forksHashMapping[fork], myGossiper.blockchain.nameHashMapping

	return rewindNumb
}

func (blockchain *Blockchain) printChain() {

	s := "CHAIN\n"
	// s :="CHAIN"

	b := myGossiper.blockchain.head
	for {
		s += b.DescribeBlock() + "\n"
		// s+= " " +b.DescribeBlock()
		if bytes.Equal(b.PrevHash[:], make([]byte, 32, 32)) {
			break
		}
		b = myGossiper.blockchain.blocks[hex.EncodeToString(b.PrevHash[:])]
		fmt.Println("Trans : ", b.Transactions)
	}
	fmt.Println(s[:len(s)-1])
	// fmt.Println(s)
}

func (block Block) DescribeBlock() string {
	currentHash := block.Hash()
	currentHashString := hex.EncodeToString(currentHash[:])
	prevHashString := hex.EncodeToString(block.PrevHash[:])

	s := currentHashString + ":" + prevHashString + ":"

	for _, tx := range block.Transactions {
		s += tx.File.Name + ","
	}

	s = s[:len(s)-1]
	return s
}

func withDrawFromPending(toWithDraw []TxPublish) {
	myGossiper.pendingTransactions.mux.Lock()
	defer myGossiper.pendingTransactions.mux.Unlock()

	for _, twd := range toWithDraw {
		for i := len(myGossiper.pendingTransactions.transactions) - 1; i >= 0; i-- {
			if myGossiper.pendingTransactions.transactions[i].File.Name == twd.File.Name {
				copy(myGossiper.pendingTransactions.transactions[i:], myGossiper.pendingTransactions.transactions[i+1:])
				myGossiper.pendingTransactions.transactions[len(myGossiper.pendingTransactions.transactions)-1] = TxPublish{}
				myGossiper.pendingTransactions.transactions = myGossiper.pendingTransactions.transactions[:len(myGossiper.pendingTransactions.transactions)-1]
				break
			}

		}

	}

}

// startMining is used to mine continously
func startMining() {

	// First block to mine
	block := Block{PrevHash: [32]byte{}, Transactions: nil}
	//start := time.Now()

	for {

		// Generate random Nonce
		nonce := [32]byte{}
		_, err := rand.Read(nonce[:])
		if err != nil {
			checkError(err, true)
		}
		block.Nonce = nonce
		//fmt.Println(nonce)

		if block.Valid() && len(block.Transactions) != 0 {
			fmt.Printf("FOUND-BLOCK %x\n", block.Hash())
			fmt.Println("With transactions: ", block.Transactions)
			myGossiper.blockChannel <- block

			//time.Sleep(10000 * time.Millisecond)

			broadcastBlock(&BlockPublish{
				HopLimit: HOP_LIMIT_BLOCK,
				Block:    block,
			}, "")

		}

		// Updating the pending transaction

		myGossiper.blockchain.mux.Lock()
		myGossiper.pendingTransactions.mux.Lock()

		block.Transactions = copieTransaction(myGossiper.pendingTransactions.transactions)
		if myGossiper.blockchain.lengthLongestChain != 0 {
			block.PrevHash = myGossiper.blockchain.head.Hash()
		}

		myGossiper.pendingTransactions.mux.Unlock()
		myGossiper.blockchain.mux.Unlock()

	}
}

func copieTransaction(txs []TxPublish) []TxPublish {
	newTx := make([]TxPublish, len(txs))
	for i := range txs {
		newTx[i] = TxPublish{
			File: File{
				Name:         txs[i].File.Name,
				Size:         txs[i].File.Size,
				MetafileHash: txs[i].File.MetafileHash},
			HopLimit: txs[i].HopLimit}
	}

	return newTx
}

// nameOk verify if the name we want to index is not
// already own by someone else or claimed by someone else
func nameOk(name string) bool {

	_, seen := myGossiper.blockchain.nameHashMapping[name]

	if seen {
		return false
	}

	for i := range myGossiper.pendingTransactions.transactions {
		if name == myGossiper.pendingTransactions.transactions[i].File.Name {
			return false
		}
	}
	return true
}

//Broadcast Tx publish to all peers except relayer
func sendTxPublish(fr *FileRecord, size int64, relayer string) {
	// Create File
	MetaByte, err := hex.DecodeString(fr.MetaHash)
	checkError(err, true)

	f := File{
		Name:         fr.Name,
		Size:         size,
		MetafileHash: MetaByte,
	}

	// Create TxPublish
	tx := TxPublish{
		File:     f,
		HopLimit: HOP_LIMIT,
	}

	sendToAllNeighbors(&GossipPacket{TxPublish: &tx}, relayer)

}
