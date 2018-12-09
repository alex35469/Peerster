package main

import (
	"net"
	"sync"
	"time"
)

// Set the time for Timer and Tickers
const TIME_OUT time.Duration = time.Second
const ANTI_ENTROPY_DURATION time.Duration = time.Second
const FILE_DURATION time.Duration = 5 * time.Second
const SEEN_SEARCH_REQUEST_TIMEOUT time.Duration = 500 * time.Millisecond
const SEARCH_CLIENT_TIMEOUT time.Duration = time.Second
const MINEUR_SLEEPING_TIME = 5 * time.Second
const ORPHAN_RESOLUTION = 800 * time.Millisecond

//###### PEERSTER MESSAGES TYPES   #######

const UDP_PACKET_SIZE = 10000
const HOP_LIMIT = 10
const HOP_LIMIT_BLOCK = 20
const MAX_BUDGET = 32
const REQUIRED_MATCH = 2
const MINING_BYTES = 2

var myGossiper *Gossiper

var UIPort, gossipAddr, name, neighborsInit string
var simple bool
var rtimer int

var stack = make([]StackElem, 0)
var infos = make([]InfoElem, 0)

// MESSAGES

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
	FileName      string
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

type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}
type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
}

type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
}

type TxPublish struct {
	File     File
	HopLimit uint32
}

type BlockPublish struct {
	Block    Block
	HopLimit uint32
}

type File struct {
	Name         string
	Size         int64
	MetafileHash []byte
}

type Block struct {
	PrevHash     [32]byte
	Nonce        [32]byte
	Transactions []TxPublish
}

type GossipPacket struct {
	Simple        *SimpleMessage
	Rumor         *RumorMessage
	Status        *StatusPacket
	Private       *PrivateMessage
	DataRequest   *DataRequest
	DataReply     *DataReply
	SearchRequest *SearchRequest
	SearchReply   *SearchReply
	TxPublish     *TxPublish
	BlockPublish  *BlockPublish
}

// COMUNICATION WITH CLIENT
type ClientMessage struct {
	File    string
	Request string
	Dest    string
}

type ClientSearch struct {
	Keywords []string
	Budget   uint64
}

type ClientPacket struct {
	Broadcast *SimpleMessage
	Private   *PrivateMessage
	CMessage  *ClientMessage
	CSearch   *ClientSearch
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

type SafeChunkToDownload struct {
	tickers []*time.Ticker
	chunks  [][]byte
	metas   [][]byte // Refer to the parent meta Hash
	dests   []string
	fname   []string
	mux     sync.Mutex
}

type FileRecord struct {
	Name     string
	NbChunk  uint64
	MetaFile []string
	MetaHash string
}

type SafeFileRecords struct {
	files []*FileRecord
	mux   sync.Mutex
}

type safeSearchesSeen struct {
	searchesSeen []*SearchRequest
	mux          sync.Mutex
}

type Gossiper struct {
	address             *net.UDPAddr
	conn                *net.UDPConn
	Name                string
	neighbors           []string
	myVC                *StatusPacket
	safeTimersRecord    SafeTimerRecord
	messagesHistory     map[string]*SafeMsgsOrginHistory
	routingTable        map[string]*RoutingTableEntry
	mux                 sync.Mutex
	safeFiles           SafeFileRecords
	safeCtd             SafeChunkToDownload
	safeSearchesSeen    safeSearchesSeen
	safeOngoingSearch   OngoingSearch
	safeDownloadingFile DownloadingFile
	safeReadyToDownload ReadyToDownload
	pendingTransactions pendingTransactions
	blockchain          Blockchain
	blockChannel        chan Block
}

type Blockchain struct {
	// The chain is represent by a map hash -> block
	blocks map[string]Block

	head               Block
	lengthLongestChain int
	nameHashMapping    map[string]string

	forksHead        []Block
	forksHashMapping []map[string]string
	forksLength      []int

	orphansBlock map[string]Block

	mux sync.Mutex
}

type pendingTransactions struct {
	transactions []TxPublish
	mux          sync.Mutex
}

type DownloadingFile struct {
	fname      []string
	chunkMap   []map[uint64]string
	chunkCount []uint64
	metaHash   []string
	mux        sync.Mutex
}

type ReadyToDownload struct {
	fname      []string
	chunkMap   []map[uint64]string
	chunkCount []uint64
	metaHash   []string
	mux        sync.Mutex
}

type OngoingSearch struct {
	tickers    []*time.Ticker
	searches   []*SearchRequest
	matches    []uint8
	seenHashes [][]string
	//seenNames  [][]string
	mux sync.Mutex
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

type InfoElem struct {
	Fname string
	Hash  string
	Event string
	Desc  string
}
