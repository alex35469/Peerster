

const backendAddr = "http://127.0.0.1:8080";
const ID_PATH = "/id";
const MESSAGES_PATH = "/message";
const NODE_PATH = "/node";
const FILE_PATH = "/file"
let select = "All";
let msgs = [];
let nodes = new Set(["All"])
let myId = ""
let stats = new Set()
let fileSelected = "None"
let filesToDownload = new Set()

/*
	Run the action when we are sure the DOM has been loaded (FROM dataviz course)
*/
function whenDocumentLoaded(action) {
	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", action);
	} else {
		action();
	}
}


function getFileName(){
	let fname =  $("#file-input").val()

	// From https://stackoverflow.com/questions/857618/javascript-how-to-extract-filename-from-a-file-input-control
	let startIndex = (fname.indexOf('\\') >= 0 ? fname.lastIndexOf('\\') : fname.lastIndexOf('/'));
    fname = fname.substring(startIndex);
    if (fname.indexOf('\\') === 0 || fname.indexOf('/') === 0) {
        fname = fname.substring(1);
    }
	return fname

}


// Send msg to backend
function fetchAndSendMessage(){
	let msg = document.getElementsByName("send-input")[0].value;
	if (msg === "") {
		return
	}


	$.post(
		backendAddr+MESSAGES_PATH,
		{Msg:msg, Dest:select},
		'jsonp'
	)


	updateChatBox()
}

// inform the backend about the new peer
function addPeer(){
	let addr = document.getElementsByName("add-input")[0].value;
	if (addr === "") {
		return
	}

	const tmp = addr.split(":")

	if (tmp.length != 2 || !(tmp[1] >= 0) || !(tmp[1] < 60000) ){
		return
	}


	const tmp2 = tmp[0].split(".");
	if(tmp2.length != 4){
		return
	}

	let unvalid = false
	tmp2.forEach(x => {

		if ( (isNaN(x)) || !(x < 256) || !(x >= 0)){
			console.log("not in the limit")
			unvalid = true

		}

	})

	if (unvalid) {
		$("#add-btn").css({background : "red"});
		return
	}

	$.post(
		"http://localhost:8080/node",
		{Addr: addr},
		'jsonp'
	)
	getNewNode()
}

var getNewNode = function(){
	$.getJSON(
		backendAddr+NODE_PATH,
		function(json) {
			$("#node-box").empty()
			json["peers"].forEach(p => {
				$("#node-box").append(p+"<br />");
			})
			json["nodes"].forEach(n => {
				updateNodeBox(n, nodes)
			})

 		}
	);

}

function updateNodeBox(newElem, nodes){
	if (!nodes.has(newElem)) {
		nodes.add(newElem)
		$("#chat-option").append("<span class='option' onclick='openChat(this.id)' id = '"+newElem+"'>"+newElem+"</span> <br />");

	}

}

function openChat(id){
	$("#selection").html(id)

	if (id == "All"){
		$("#selection2").html("None")
	}

	else{
		$("#selection2").html(id)
	}

	$("#"+id).html(id)
	select = id
	updateChatBox()
}

function fileSelector(name){
	$("#file-selected").html($("#"+name).text())
	fileSelected = name

}



var getNewMsg = function(){
	// get new msgs
	$.get(
		"http://localhost:8080/message",
		function(json) {
			$("#chat-box")
			json.msgs.forEach(m => {



				if (m !== "") {

					msgs.push({'origin':m["Origin"], 'msg':m["Msg"], "dest":m["Dest"], "mode":m["Mode"]})

					if (m["Mode"] == "All" && select != "All"){
						$("#All").html("All    <span style='color: #ff0000;text-align=right'>New Messages</spane>")
					}

					if (m["Mode"] == "Private" && select != m["Origin"]){
						$("#"+m["Origin"]).html(m["Origin"]+"  <span style='color: #ff0000;text-align=right'>New Messages</spane>")
					}

				}
			})
		}
	)
}


var updateChatBox = function(){
	$("#chat-box").empty()
	msgs.forEach(e => {
		if (e.origin === select && e.mode === "Private" || select == "All" &&  e.mode !=  "Private") {
			if (e.origin != myId){
				$("#chat-box").append(e.origin +" : "+ e.msg +"<br />");
			}
		}
		if (e.origin == myId && (e.dest == select || e.mode == select)){
			$("#chat-box").append(e.origin +" : "+ e.msg +"<br />");
		}

		}
	)

}

var getPeerId = function(){
	// Fetch the Peer ID
	$.getJSON(
		backendAddr+ID_PATH,
		function(json) {
			document.getElementById("peerID").innerText = json.ID;
			document.getElementById("addr").innerText = json.addr;
			myId = json.ID
		}
	);

}








whenDocumentLoaded(() => {

	getPeerId()
	getNewNode()

	setInterval(getPeerId, 2*1000);

	setInterval(getNewNode, 2*1000);
	setInterval(getNewMsg, 2*1000);
	setInterval(updateChatBox, 1*1000);
	setInterval(getInfos, 2*1000);




	// Post new node
	let add = document.getElementById("add-btn");
	add.addEventListener("click", () => addPeer())


	// Post new messages
	let send = document.getElementById("send-btn");
	send.addEventListener("click", () => fetchAndSendMessage())


	// In case we want to post the entire file to the gossiper
	/*
	let form = document.getElementById('file-form');
	let fileSelect = document.getElementById('file-select');
	let uploadButton = document.getElementById('upload-button');



	form.onsubmit = function(event) {
		// Create a new FormData object.
		let formData = new FormData();
		event.preventDefault();
		uploadButton.innerHTML = 'sharing...';
		let file = fileSelect.files[0];
		console.log("file name  = " + file.name);
		console.log("file type  = " + file.type);

		formData.append("name", file, file.name); //, file.name)

		for (var [key, value] of formData.entries()) {
  		console.log("Key = ",  key, "Value = ", value);
		}
		console.log(formData)


		// Set up the request.
		let xhr = new XMLHttpRequest();

		// Open the connection.
		xhr.open('POST', backendAddr+FILE_PATH, true);

		// Set up a handler for when the request finishes.
		xhr.onload = function () {
  		if (xhr.status === 200) {
    		// File(s) uploaded.
    		uploadButton.innerHTML = 'Upload';
  		} else {
    		alert('An error occurred!');
  		}
		};

		// Send the Data.
		xhr.send(formData);



	}
	*/

	let share = document.getElementById("share-btn");
	share.addEventListener("click", () => shareFile())


	let download = document.getElementById("download-btn");
	download.addEventListener("click", () => downloadFile())

	let search = document.getElementById("search-btn");
	search.addEventListener("click", () => searchFile())

	let downloadable = document.getElementById("downloadable-btn");
	downloadable.addEventListener("click", () => downloadSelected())



});


function downloadSelected(){
	if (fileSelected == "None"){
		return
	}
	$("#"+name).text()

	$.post(
		backendAddr+FILE_PATH,
		{name:$("#"+fileSelected).text(), metaHash:fileSelected ,mode:"downloadable"},
		'jsonp'
	)
	$("#info-box").append("Downloading "+fileSelected +" ...<br />");

}


function downloadFile(){
	let fname = document.getElementsByName("fname-input")[0].value;
	let hash = document.getElementsByName("hash-input")[0].value;



	if (select == "All" ){
		$("#info-box").append("Please select a valide node <br />")
		return
		}
	if(fname == "" ){
		$("#info-box").append("Please enter the file name <br />")
		return
	}

	if(fname.indexOf(' ') >= 0){
    $("#info-box").append("No white space allowed<br />");
		return
	}

	let re = /[0-9a-f]{64}/
	if  (!re.test(hash)){
		$("#info-box").append("Not conform to sha256 hexaHash format <br />");
		return

	}

	$.post(
		backendAddr+FILE_PATH,
		{name:fname, metaHash:hash, dest:select , mode:"download"},
		'jsonp'
	)
	$("#info-box").append("Downloading "+fname +" from "+select + "...<br />");

}



function shareFile(){

	let fname = getFileName()
	if (fname == ""){
		return
	}
	console.log("Shareing " + fname)

	$.post(
		backendAddr+FILE_PATH,
		{name:fname, mode:"share"},
		'jsonp'
	)
	$("#info-box").append("Sharing "+fname +"...<br />");
}

function getInfos(){
	// Fetch the Peer ID
	$.get(
		"http://localhost:8080/file",
		function(json) {
			json.infos.forEach(info => {

				//console.log(info["Event"])



				if (info["Event"] == "error") {
					$("#info-box").append("Error "+info["Fname"]+ " "+ info["Desc"]+"<br />");
					return
				}

				if (info["Event"] == "download") {
					$("#info-box").append(info["Fname"] + " "+  info["Desc"]+"<br />");
					return

				}

				if (info["Event"] == "indexed"){

					$("#info-box").append(info["Fname"] + " "+  info["Desc"]+ " " + "<span style='color: #ff0000;text-align=right'>"+info["Hash"]+ "</span><br />");
					return
				}

				if (info["Event"] == "search"){
					$("#search-box").append(info["Desc"] +"...<br />");

				}

				if (info["Event"] == "downloadable"){
					$("#search-box").append(info["Desc"] +".<br />");

					// See if we already have the same hash:
					if (filesToDownload.has(info["Hash"])){
						$("#"+info["Hash"]).html(info["Fname"])
						return
					}
					filesToDownload.add(info["Hash"])
					$("#downloadable-box").append("<span class='option' onclick='fileSelector(this.id)' id = '"+info["Hash"]+"'>"+info["Fname"]+"</span>, ")


				}

			})
		}
	)


}

function searchFile(){
	let keywords = document.getElementsByName("keywords-input")[0].value;
	let budget = document.getElementsByName("budget-input")[0].value;

	if (keywords == ""){
		return
	}

	$.post(
		backendAddr+FILE_PATH,
		{mode:"search", keywords: keywords, budget:budget},
		'jsonp'
	)



}


// Helpers functions

// from https://stackoverflow.com/questions/1527803/generating-random-whole-numbers-in-javascript-in-a-specific-range


/* 		const newContent = document.createTextNode(t);
		d.appendChild(newContent);
		container_element.appendChild(d); */
