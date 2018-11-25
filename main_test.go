package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestScanFileAndCheckSum(t *testing.T) {

	myGossiper := NewGossiper("127.0.0.1:6000", "nodeTest", "127.0.0.1:6002,127.0.0.1:6003,127.0.0.1:6004")
	rf1, _ := ScanFile("file.txt")
	fmt.Println(rf1)

	rf2, _ := ScanFile("carlton.txt")
	fmt.Println(rf2)

	rf3, _ := ScanFile("im1.jpg")
	fmt.Println(rf3)

	myGossiper.safeFiles.files = append(myGossiper.safeFiles.files, rf1)
	myGossiper.safeFiles.files = append(myGossiper.safeFiles.files, rf2)
	myGossiper.safeFiles.files = append(myGossiper.safeFiles.files, rf3)
	b1, _ := hex.DecodeString("129fe7a41d63d5d2dbc89816ee86298a04dd47654dbe210a09073bd24ae40729")

	i, j := chunkSeek(b1, myGossiper)

	if i != 2 || j != -1 {
		t.Errorf("Expected i = 2 and j = -1 but got i = %d and j=%d ", i, j)
	}

	b2, _ := hex.DecodeString("8eca4615e2c45366a822f1e732ae1bea9f7aecaf5dcc58c3dae6da2b6ed1dbb5")
	i, j = chunkSeek(b2, myGossiper)

	if i != 2 || j != 7 {
		t.Errorf("Expected i = 2 and j = 7 but got i = %d and j=%d ", i, j)
	}

	b3, _ := hex.DecodeString("dc179d1241faa12394d81ce16168fb9b8eb6e123bec0afa139e3f10464b07554")

	i, j = chunkSeek(b3, myGossiper)
	if i != -1 {
		t.Errorf("Expected i = -1 but got i = %d", i)
	}

	b4, _ := hex.DecodeString("dc179d1243faa12394d81ce16168fb9b8eb6e123bec0afa139e3f10464b07554")

	i, j = chunkSeek(b4, myGossiper)
	if i != 0 || j != 1 {
		t.Errorf("Expected i = 0 and j = 1 but got i = %d and j = %d", i, j)
	}

	/*
		data, h := getDataAndHash(i, j, myGossiper)
		if !bytes.Equal(b4, h) {
			t.Errorf("Expected h = %x but got h = %x", b4, h)
		}
		s := hex.EncodeToString(data)
		if s != "5b8f8878a3f398fa93f591f2f0ae69321b88897ea096d8c6720f827df81810a1dc179d1243faa12394d81ce16168fb9b8eb6e123bec0afa139e3f10464b07554" {
			t.Errorf("Expected s = ea0187742b9be0a5b717b118ec7c2b36c3e91d789bad272cca817348416c377c but got s = %s", s)
		}

		d := "492540a77c76bb091370aee42cd2c0b5f5ba4c4520e7e3492c46a852f690831645e9baede15bc785096188994799ed7bee54c56e1709a1398c2d9c65bedfd0a0e8bf62332041cca560c69ca7257aabe8cd530e19f2e342f6de5f1c0fb1086166066268c93bca6630ce1e08d3077bae5c693b9144cd037858cff8d00e3d8fa4ed"
		r := "c8c557298b82082e5ef660d470aa897bcce552ba81658070f687bb264376c8b8"
		rByte, _ := hex.DecodeString(r)

		i, j = chunkSeek(rByte, myGossiper)
		data, h = getDataAndHash(i, 10, myGossiper)

		dreturn := hex.EncodeToString(data)
		if d != dreturn {
			t.Errorf("Expected dreturn = %s but got %s ", dreturn, d)
		}

		// CHECK TO REQUEST A CHUNK
		rf4, _ := ScanFile("Germany.txt")
		addFileRecord(rf4, myGossiper)
		if len(myGossiper.safeFiles.files) != 3 {
			t.Error("Expected len(myGossiper.files) = 3 but got ", len(myGossiper.safeFiles.files))
		}

	*/

}

func TestEqualityCheckRecievedData(t *testing.T) {
	b1 := make([]byte, 3)
	b1[0] = 1
	b1[1] = 2
	b1[2] = 3
	sha := sha256.New()
	sha.Write(b1)
	h := sha.Sum(nil)

	if !EqualityCheckRecievedData(h, b1) {
		t.Error("Expected true but got false")
	}

	b3 := make([]byte, 3)
	b3[0] = 1
	b3[1] = 4
	b3[2] = 3
	if EqualityCheckRecievedData(h, b3) {
		t.Error("Expected false but got true")
	}

}

// Test for comparing 2 packet status

func TestCreateGossiper(t *testing.T) {

	testGossiper := NewGossiper("127.0.0.1:6001", "nodeTest", "127.0.0.1:6002,127.0.0.1:6003,127.0.0.1:6004")

	if testGossiper.Name != "nodeTest" {
		t.Error("Expected nodeTest but got ", testGossiper.Name)
	}

}

func TestCompareStatusPacket(t *testing.T) {

	// Init Test
	sp1 := StatusPacket{}
	sp2 := StatusPacket{}

	outcome, id, next := sp1.CompareStatusPacket(&sp2)

	if outcome != 0 {
		t.Error("Init Test: Expected outcome 0 (VC are the same) got", outcome)
	}

	// We are in advance and the other one has nothing yet
	want1 := []PeerStatus{
		PeerStatus{Identifier: "A", NextID: 21},
	}
	sp1 = StatusPacket{Want: want1}

	outcome, id, next = sp1.CompareStatusPacket(&sp2)

	if outcome != 1 || id != "A" || next != 1 {
		t.Errorf("Init Test: Expected outcome = 1, Identifier = A, NextID = 0 but got (%v, %v, %v) respectively", outcome, id, next)
	}

	outcome, id, next = sp2.CompareStatusPacket(&sp1)

	if outcome != -1 {
		t.Error("Should be late but got Case = ", outcome)
	}

	want2 := []PeerStatus{
		PeerStatus{Identifier: "A", NextID: 21},
		PeerStatus{Identifier: "B", NextID: 2},
	}
	sp1 = StatusPacket{Want: want1}
	sp2 = StatusPacket{Want: want2}
	outcome, _, _ = sp1.CompareStatusPacket(&sp2)
	if outcome != -1 {
		t.Error("Expected outcome -1 (We are late) but got", outcome)
	}

	sp1.Want = append(sp1.Want, PeerStatus{Identifier: "B", NextID: 2})

	outcome, _, _ = sp1.CompareStatusPacket(&sp2)
	if outcome != 0 {
		t.Error("Expected outcome 0 (We are late) got", outcome)
	}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "C", NextID: 34},
	)

	sp2.Want = append(sp2.Want,
		PeerStatus{Identifier: "C", NextID: 13},
		PeerStatus{Identifier: "E", NextID: 20},
	)

	outcome, id, next = sp1.CompareStatusPacket(&sp2)

	if outcome != 1 || id != "C" || next != 13 {

		t.Errorf("Expected outcome = 1, Identifier = C, NextID = 13 but got (%v, %v, %v) respectively", outcome, id, next)
	}

	outcome, id, next = sp2.CompareStatusPacket(&sp1)
	if outcome != 1 || id != "E" || next != 1 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 1, Identifier = D, NextID = 1 but got (%v, %v, %v) respectively", sp2, sp1, outcome, id, next)
	}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "D", NextID: 10},
	)

	outcome, id, next = sp2.CompareStatusPacket(&sp1)
	if outcome != 1 || id != "E" || next != 1 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 1, Identifier = E, NextID = 1 but got (%v, %v, %v) respectively", sp2, sp1, outcome, id, next)
	}

	sp1 = StatusPacket{}
	sp2 = StatusPacket{}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "B", NextID: 4},
		PeerStatus{Identifier: "D", NextID: 3},
		PeerStatus{Identifier: "R", NextID: 6},
	)

	sp2.Want = append(sp2.Want,
		PeerStatus{Identifier: "B", NextID: 6},
		PeerStatus{Identifier: "C", NextID: 4},
		PeerStatus{Identifier: "D", NextID: 3},
		PeerStatus{Identifier: "E", NextID: 2},
		PeerStatus{Identifier: "R", NextID: 5},
	)

	outcome, id, next = sp1.CompareStatusPacket(&sp2)
	if outcome != 1 || id != "R" || next != 5 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 1, Identifier = E, NextID = 0 but got (%v, %v, %v) respectively", sp1, sp2, outcome, id, next)
	}

	sp1 = StatusPacket{}
	sp2 = StatusPacket{}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "B", NextID: 4},
		PeerStatus{Identifier: "F", NextID: 3},
		PeerStatus{Identifier: "R", NextID: 6},
	)

	sp2.Want = append(sp2.Want,
		PeerStatus{Identifier: "B", NextID: 6},
		PeerStatus{Identifier: "C", NextID: 4},
		PeerStatus{Identifier: "D", NextID: 3},
		PeerStatus{Identifier: "E", NextID: 2},
		PeerStatus{Identifier: "G", NextID: 5},
	)

	outcome, id, next = sp1.CompareStatusPacket(&sp2)
	if outcome != 1 || id != "F" || next != 1 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 1, Identifier = F, NextID = 1 but got (%v, %v, %v) respectively", sp1, sp2, outcome, id, next)
	}

	sp1 = StatusPacket{}
	sp2 = StatusPacket{}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "B", NextID: 4},
		PeerStatus{Identifier: "F", NextID: 3},
		PeerStatus{Identifier: "T", NextID: 10},
	)

	sp2.Want = append(sp2.Want,
		PeerStatus{Identifier: "B", NextID: 4},
		PeerStatus{Identifier: "F", NextID: 3},
		PeerStatus{Identifier: "R", NextID: 6},
	)

	outcome, id, next = sp1.CompareStatusPacket(&sp2)
	if outcome != 1 || id != "T" || next != 1 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 1, Identifier = T, NextID = 1 but got (%v, %v, %v) respectively", sp1, sp2, outcome, id, next)
	}

	sp1 = StatusPacket{}
	sp2 = StatusPacket{}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "P", NextID: 7},
		PeerStatus{Identifier: "H", NextID: 7},
	)

	sp2.Want = append(sp2.Want,
		PeerStatus{Identifier: "A", NextID: 1},
		PeerStatus{Identifier: "B", NextID: 1},
		PeerStatus{Identifier: "P", NextID: 7},
		PeerStatus{Identifier: "H", NextID: 7},
		PeerStatus{Identifier: "Z", NextID: 1},
	)

	outcome, id, next = sp1.CompareStatusPacket(&sp2)
	if outcome != 0 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 0 but got %v", sp1, sp2, outcome)
	}

	sp1 = StatusPacket{}
	sp2 = StatusPacket{}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "B", NextID: 4},
		PeerStatus{Identifier: "D", NextID: 6},
	)

	sp2.Want = append(sp2.Want,
		PeerStatus{Identifier: "A", NextID: 3},
		PeerStatus{Identifier: "B", NextID: 4},
		PeerStatus{Identifier: "D", NextID: 6},
	)

	outcome, id, next = sp1.CompareStatusPacket(&sp2)
	if outcome != -1 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = -1, Identifier = '', NextID = 0 but got (%v, %v, %v) respectively", sp1, sp2, outcome, id, next)
	}

}

func TestSortVC(t *testing.T) {
	sp1 := StatusPacket{}
	sp1Correct := StatusPacket{}

	sp2 := StatusPacket{}
	sp2Correct := StatusPacket{}

	sp3 := StatusPacket{}
	sp3Correct := StatusPacket{}

	sp1.SortVC()

	sp3.Want = append(sp3.Want,
		PeerStatus{Identifier: "A", NextID: 6},
		PeerStatus{Identifier: "B", NextID: 3},
		PeerStatus{Identifier: "C", NextID: 4},
		PeerStatus{Identifier: "D", NextID: 54},
	)

	sp3Correct.Want = append(sp3Correct.Want,
		PeerStatus{Identifier: "A", NextID: 6},
		PeerStatus{Identifier: "B", NextID: 3},
		PeerStatus{Identifier: "C", NextID: 4},
		PeerStatus{Identifier: "D", NextID: 54},
	)

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "D", NextID: 4},
		PeerStatus{Identifier: "A", NextID: 6},
	)

	sp1Correct.Want = append(sp1Correct.Want,
		PeerStatus{Identifier: "A", NextID: 6},
		PeerStatus{Identifier: "D", NextID: 4},
	)

	sp2.Want = append(sp2.Want,
		PeerStatus{Identifier: "B", NextID: 3},
		PeerStatus{Identifier: "R", NextID: 54},
		PeerStatus{Identifier: "C", NextID: 4},
		PeerStatus{Identifier: "A", NextID: 6},
	)

	sp2Correct.Want = append(sp2Correct.Want,
		PeerStatus{Identifier: "A", NextID: 6},
		PeerStatus{Identifier: "B", NextID: 3},
		PeerStatus{Identifier: "C", NextID: 4},
		PeerStatus{Identifier: "R", NextID: 54},
	)

	sp1.SortVC()

	for i, _ := range sp1.Want {

		if sp1.Want[i].Identifier != sp1Correct.Want[i].Identifier {
			t.Errorf("Expected %v but got %v after the sort", sp1, sp1Correct)
		}
	}

	sp2.SortVC()
	for i, _ := range sp2.Want {

		if sp2.Want[i].Identifier != sp2Correct.Want[i].Identifier {
			t.Errorf("Expected %v but got %v after the sort", sp2, sp2Correct)
		}
	}

	sp3.SortVC()
	for i, _ := range sp3.Want {

		if sp3.Want[i].Identifier != sp3Correct.Want[i].Identifier {
			t.Errorf("Expected %v but got %v after the sort", sp3, sp3Correct)
		}
	}

}
