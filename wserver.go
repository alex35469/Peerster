package main

import (

  "reflect"
  "fmt"
  "net/http"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// ############################# WEBSERVER  and connection ##########################

func sendID(w http.ResponseWriter, r *http.Request) {
	json := simplejson.New()
	json.Set("ID", myGossiper.Name)
	json.Set("addr", myGossiper.address.String())

	payload, err := json.MarshalJSON()

	checkError(err)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)

}

func msgsPost(w http.ResponseWriter, r *http.Request) {
	// from https://stackoverflow.com/questions/15672556/handling-json-post-request-in-go
	content := r.FormValue("Msg")
	dest := r.FormValue("Dest")

	fmt.Println(dest)
	fmt.Println(content)
	if dest == "All" {
		packet := &GossipPacket{Simple: &SimpleMessage{OriginalName: "Client", RelayPeerAddr: "Null", Contents: content}}
		processMsgFromClient(packet)
	} else {
		packet := PrivateMessage{Text: content, Destination: dest}
		processMsgFromClient(&GossipPacket{Private: &packet})

	}

	//msgsGet(w, r)

}



func msgsGet(w http.ResponseWriter, r *http.Request) {

	json := simplejson.New()
	json.Set("msgs", stack)

	// flush the stack
	stack = make([]StackElem, 0)

	payload, err := json.MarshalJSON()
	checkError(err)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}

func nodePost(w http.ResponseWriter, r *http.Request) {
	// from https://stackoverflow.com/questions/15672556/handling-json-post-request-in-go

	body := r.FormValue("Addr")
	//checkError(err)

	addNeighbor(string(body))

	nodeGet(w, r)

}

func nodeGet(w http.ResponseWriter, r *http.Request) {
	json := simplejson.New()
	json.Set("peers", myGossiper.neighbors)

	// Retrieved from https://stackoverflow.com/questions/41690156/how-to-get-the-keys-as-string-array-from-map-in-go-lang/41691320
	keys := reflect.ValueOf(myGossiper.routingTable).MapKeys()
	nodes := make([]string, 0)
	for i := 0; i < len(keys); i++ {
		if keys[i].String() != myGossiper.Name {
			nodes = append(nodes, keys[i].String())
		}
	}

	json.Set("nodes", nodes)

	payload, err := json.MarshalJSON()
	checkError(err)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}

func shareFile(w http.ResponseWriter, r *http.Request) {
  fmt.Println("Good path")
  fname := r.FormValue("name")
  mode := r.FormValue("mode")
  fmt.Println(fname)
  fmt.Println(mode)

}

func listenToGUI() {

	r := mux.NewRouter()
	r.HandleFunc("/id", sendID).Methods("GET")
	r.HandleFunc("/message", msgsPost).Methods("POST")
	r.HandleFunc("/message", msgsGet).Methods("GET")
	r.HandleFunc("/node", nodePost).Methods("POST")
	r.HandleFunc("/node", nodeGet).Methods("GET")
  r.HandleFunc("/file", shareFile).Methods("POST")

	http.Handle("/", r)

	http.ListenAndServe(":8080", handlers.CORS()(r))
}
