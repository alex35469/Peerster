package main

import (
	"fmt"
	"net/http"
	"os"
	"reflect"

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

	checkError(err, false)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)

}

func msgsPost(w http.ResponseWriter, r *http.Request) {
	// from https://stackoverflow.com/questions/15672556/handling-json-post-request-in-go
	r.ParseForm()
	content := r.FormValue("Msg")
	dest := r.FormValue("Dest")

	fmt.Println(dest)
	fmt.Println(content)
	if dest == "All" {
		packet := &ClientPacket{Broadcast: &SimpleMessage{OriginalName: "Client", RelayPeerAddr: "Null", Contents: content}}
		processMsgFromClient(packet)
	} else {
		packet := PrivateMessage{Text: content, Destination: dest}
		processMsgFromClient(&ClientPacket{Private: &packet})

	}

	//msgsGet(w, r)

}

func msgsGet(w http.ResponseWriter, r *http.Request) {

	json := simplejson.New()
	json.Set("msgs", stack)

	// flush the stack
	stack = make([]StackElem, 0)

	payload, err := json.MarshalJSON()
	checkError(err, false)
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
	checkError(err, false)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}

func shareFile(w http.ResponseWriter, r *http.Request) {

	fname := r.FormValue("name")
	hash := r.FormValue("metaHash")
	mode := r.FormValue("mode")
	dest := r.FormValue("dest")

	if mode == "share" {

		if _, err := os.Stat("./_SharedFiles/" + fname); os.IsNotExist(err) {
			fmt.Println("The file doesn't exist in the shared folder")
		} else {
			rf, err := ScanFile(fname)

			if err != nil {
				fmt.Println(err.Error())
				infos = append(infos, InfoElem{Fname: fname, Event: "error", Desc: err.Error(), Hash: ""})
				return
			}

			infos = append(infos, InfoElem{Fname: fname, Event: "indexed", Desc: "MetaHash = ", Hash: rf.MetaHash})

			err = addFileRecord(rf, myGossiper)

			if err != nil {
				infos = append(infos, InfoElem{Fname: fname, Event: "duplicate", Desc: err.Error(), Hash: ""})
			}
		}
	}

	if mode == "download" {
		cm := ClientMessage{
			File:    fname,
			Request: hash,
			Dest:    dest,
		}
		processMsgFromClient(&ClientPacket{CMessage: &cm})

	}

	// In case we want to upload the file
	/*
		  r.ParseForm()
			fmt.Println(r.MultipartForm)
			fname := r.FormValue("name")
			mode := r.FormValue("mode")
			fmt.Println(fname)
			fmt.Println(mode)

		  fmt.Println("Good path")
		  fmt.Println(r.ContentLength)
		  fmt.Println(r.Header["Content-Type"]["boundary"])

			buf := new(bytes.Buffer)
			buf.ReadFrom(r.Body)

		  mp := r.MultipartReader()

		  p, err := mp.NextPart()
			b := buf.Bytes()
			s := *(*string)(unsafe.Pointer(&b))
			fmt.Println(s)
	*/

}

// Provide a feedback
func fileInfo(w http.ResponseWriter, r *http.Request) {
	json := simplejson.New()
	json.Set("infos", infos)

	// flush infos
	infos = make([]InfoElem, 0)

	payload, err := json.MarshalJSON()
	checkError(err, false)
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)

}

func listenToGUI() {

	r := mux.NewRouter()
	r.HandleFunc("/id", sendID).Methods("GET")
	r.HandleFunc("/message", msgsPost).Methods("POST")
	r.HandleFunc("/message", msgsGet).Methods("GET")
	r.HandleFunc("/node", nodePost).Methods("POST")
	r.HandleFunc("/node", nodeGet).Methods("GET")
	r.HandleFunc("/file", shareFile).Methods("POST")
	r.HandleFunc("/file", fileInfo).Methods("Get")

	http.Handle("/", r)

	http.ListenAndServe(":8080", handlers.CORS()(r))
}
