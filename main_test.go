package main

import "testing"

func TestScanFile(t *testing.T) {

	ScanFile("file.txt")
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
