// Gossiper Main file. Accept the following arguments:
//  - UIPort  : port of the UI client (default "8080")
//  - gossipAddr  : ip:port for the gossiper (default "127.0.0.1:5000")
//  - name : name of the gossiper
//  - peers : coma separated list of peers of the form ip:UIPort
//  - simple : run gossiper in simple broadcast modified

package main

import (
  "fmt"
  "flag"
//  "github.com/dedis/protobuf"
//  "net"
)

/*
type Gossiper struct{
  address string
  conn string
  Name string
}

func NewGossiper(address, name string) *Gossiper {
  udpAddr, err := net.ResolveUDPAddr("udp4", address)
  udpConn, err := net.ListenUDP("udp4", udpAddr)
  return &Gossiper{
    address: udpAddr,
    conn: udpConn,
    Name: name,
  }
}
*/

func main() {

  // Fetching the flags from the CLI
  UIPortPtr := flag.String("UIPort", "8080", "port of the UI client" )
  gossipAddrPtr := flag.String("gossipAddr", "127.0.0.1", "ip:port for the gossiper")
  namePtr := flag.String("name", "","name of the gossiper")
  peersPtr := flag.String("peers", "","name of the gossiper")
  simplePtr := flag.Bool("simple", true, "run gossiper in simple broadcast modified")


  flag.Parse()




}
