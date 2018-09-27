// Client Main file. Accept the following arguments:
//  - UIPort  : port on which the gossiper listen (default "8080")
//  - msg : message to be sent

package main

import (
  "fmt"
  "flag"
  //"github.com/dedis/protobuf"
)

// Peer simple message
type SimpleMessage struct {
  OriginalName string
  RelayPeerAddr string
  Contents string
}

type GossipPacket struct {
 Simple *SimpleMessage
}




func main(){
  UIPortPtr := flag.String("UIPort", "8080", "port of the UI client" )
  msgPtr := flag.String("msg","", "message to be sent")

  //packetToSend := GossipPacket{Simple: SimpleMessage}
  fmt.Println(UIPortPtr, msgPtr)
}
