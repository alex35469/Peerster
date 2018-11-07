package main

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"time"
)

// Code retrieved from https://astaxie.gitbooks.io/build-web-application-with-golang/en/08.1.html
// Used to ckeck if an error occured
func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", os.Stderr, err.Error())
		os.Exit(1)
	}
}

// Pick one neighbor (But not the exception) we don't want to send back a rumor
// to the one that made us discovered it for exemple
func pickOneNeighbor(exception string) string {

	var s = rand.NewSource(time.Now().UnixNano())
	var R = rand.New(s)

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

func (myVC *StatusPacket) SortVC() {

	// Since SliceSort use quick sort (best case O(n)) it's better to check if it's already sorted
	if sort.IsSorted(myVC) {
		return
	}

	sort.Sort(myVC)

}
