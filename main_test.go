package main

import "testing"

// Test for comparing 2 packet status
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

	if outcome != 1 || id != "A" || next != 0 {
		t.Errorf("Init Test: Expected outcome = 0, Identifier = A, NextID = 0 but got (%v, %v, %v) respectively", outcome, id, next)
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
	if outcome != 1 || id != "E" || next != 0 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 1, Identifier = D, NextID = 0 but got (%v, %v, %v) respectively", sp2, sp1, outcome, id, next)
	}

	sp1.Want = append(sp1.Want,
		PeerStatus{Identifier: "D", NextID: 10},
	)

	outcome, id, next = sp2.CompareStatusPacket(&sp1)
	if outcome != 1 || id != "E" || next != 0 {

		t.Errorf("%v receives \n             %v  ===> Expected outcome = 1, Identifier = E, NextID = 0 but got (%v, %v, %v) respectively", sp2, sp1, outcome, id, next)
	}

}

func TestCreateGossiper(t *testing.T) {

	testGossiper := NewGossiper("127.0.0.1:6001", "nodeTest", "127.0.0.1:6002,127.0.0.1:6003,127.0.0.1:6004")

	if testGossiper.Name != "nodeTest" {
		t.Error("Expected nodeTest but got ", testGossiper.Name)
	}

}
