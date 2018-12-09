# Peerster

Repository of the Decentralized System Engineering Course Project: Peerster | EPFL
author: Alexandre Dumur


## Files description
- ./main.go : the Gossiper programme main (backend)
- ./webserver.go : the webserver part
- ./fshare.go : the file sharing part
- ./utils.go : utils functions
- ./client/main.go : the CLI client programme (frontend)
- ./peerster.html : ./peerster.js ./style css : the GUI client interface (frontend)
- ./blockchain.go : implementation of the Nakamoto Consensus based blockchain
- ./main_test.go : use to test that some of the functions in main.go work as expected
- ./struct.go : All struct and
- ./dprocess.go : Handle data Request & data Reply messages
- ./msg.go : Handle RumorMessage & Private messages

## Set Up

### Set Up Gossiper programme
Please, run `go build` to create the executable
file and then run ./Peerster with the approptiate flags.




### Set up CLI client programme

Please, run `go build` in the client directory to create the executable
file and then run ./Client with the approptiate flags.


### Set up GUI client Interface


After having run the Gossiper programme, please open ./peerster.html
(better on Google Chrome or Safari). Make sure to press `Send` to broadcast
a message (not keypress enter). Same thing to add a peer and sharing a file. If the chat box is overloaded,
to see the most recent messages, please scroll down inside the box. Same for the node box and info-box. If the message doesn't show up right away, please, just wait 2 seconds.

Scroll down the page to see the downloading part and the file searching part



## Discussion about tests


### Rumor messages

the flipped coin test doesn't always pass because we will never send again a rumor to a peer that
just confirm he received it (StatusPacket as an Ack). We will instead choose another peer.


### File download

While Downloading the same file from 2 different locations let's say A and B, if we receive a chunk from B, then we update in C the next chunk requested as well.



### File Search
Printing SEARCH FINISHED only if we discover two files that have different Metahash (not relying on the name)


Search result with a fixed budget will be timed out (as well as the one with budget 32 when finishing the doubbling budget process). (The time out selected is 1 sec, we also considered to select this time out proportionally to the budget, but we kept 1s as for the doubbling budget process)
We motivate this by the fact that a search if it comes too late it wouldn’t mean anything anymore for the user.
And if it's not successful the User can always do the same request later in time.
Also it is not possible to perform a research with the same keywords as another one that has not finished yet.
If we have only one match still make the matched file downloadable for the User after the search is finished. (Even though we didn't print `SEARCH FINISHED`)


Since we did use the fact that the same data request addressed to different nodes have to be consistent (as mentioned above), we simply extract from the chunkMap the destinations and start download instances from each of them


### Blockchain

A minor mine and append to the blockchain blocks with empty and non empty transactions.

resolving the blocks with unknown parents are done every 1 sec. This is done by iterating over all the orphans blocks

If a chain in the belonging to the orphan block creates the longest chain, we simply ignore it until that chain found the genesis block (prevHash=0) only then that chain can be moved on on the blockchain and become the current longest.
