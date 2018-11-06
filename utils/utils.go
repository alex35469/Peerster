package utils

import(
  "net"
  "time"
  "sync"
  "os"
  "fmt"

)

// Peer simple message
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

type GossipPacket struct {
	Simple  *SimpleMessage
	Rumor   *RumorMessage
	Status  *StatusPacket
	Private *PrivateMessage
}

// Struct used to fetch a rumor when
// Stopping
type TimerForAck struct {
	rumor RumorMessage
	timer *time.Timer
}

type SafeTimerRecord struct {
	timersRecord map[string][]*TimerForAck
	mux          sync.Mutex
}

// The list
type SafeMsgsOrginHistory struct {
	history []*RumorMessage
	mux     sync.Mutex
}

// The Gossiper struct.
// address : address of the Gossiper
// connexion : connexion through which the gossiper speaks and listen
// Name used to identify his own messages
// neighbors : Peers that are "Onlink" i.e. We know their IP address
// messagesHistory : Will eventually countain all the messages from all peers in the net.
//									All the records from different origin can be locked independently due to SafeMsgsOrginHistory
// safeTimersRecord : Records of the timers regarding a particular rumor packet Can be locked

type Gossiper struct {
	Address          *net.UDPAddr
	Conn             *net.UDPConn
	Name             string
	Neighbors        []string
	MyVC             *StatusPacket
	SafeTimersRecord SafeTimerRecord
	MessagesHistory  map[string]*SafeMsgsOrginHistory
	RoutingTable     map[string]*RoutingTableEntry
	Mux              sync.Mutex
}

type RoutingTableEntry struct {
	link     string
	freshest uint32
}

type StackElem struct {
	Msg    string
	Origin string
	Dest   string
	Mode   string
}

// Code retrieved from https://astaxie.gitbooks.io/build-web-application-with-golang/en/08.1.html
// Used to ckeck if an error occured
func CheckError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", os.Stderr, err.Error())
		os.Exit(1)
	}
}
