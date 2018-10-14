# Peerster

Repository of the Decentralized System Engineering Course Project: Peerster | EPFL
author: Alexandre Dumur


## Files description
- ./main.go : the Gossiper programme (backend)
- ./client/main.go : the CLI client programme (frontend)
- ./peerster.html ./peerster.js ./style css : the GUI client interface (frontend)
- ./main_test.go use to test that some of the functions in main.go work as expected


## Set Up

### Set Up Gossiper programme
Please, run `go build` to create the executable
file and then run ./Peerster with the approptiate flags.

Noticed that the programme needs to have at least one peer at bootstrap.



### Set up CLI client programme

Please, run `go build` in the client directory to create the executable
file and then run ./Client with the approptiate flags.


### Set up GUI client Interface

After having run the Gossiper programme, please open ./peerster.html
(better on Google Chrome or Safari). Make sure to press `Send` to broadcast
a message (not keypress enter). Same thing to add a peer. If the chat box is overloaded,
to see the most recent messages, please scroll down inside the box. Same for the node box. If the message doesn't show up right away, please, just wait 2 seconds.

The GUI client communicate with the backend using HTTP on port 8080.



## Discussion about tests

the flipped coin test doesn't pass because we will never send again a rumor to a peer that
just confirm he received it (StatusPacket as an Ack). We will instead choose another peer.

test_1_ring.sh doesn't pass when the web server is active... If it is commented (All the section: Webserver connexion plus `go listenToGUI()` in main) test_1_ring.sh passes the test.
