

const backendAddr = "http://127.0.0.1:8080";
const ID_PATH = "/id";
const MESSAGES_PATH = "/message"
const NODE_PATH = "/node"


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



// Send msg to backend
function fetchAndSendMessage(){
	let msg = document.getElementsByName("send-input")[0].value;
	if (msg === "") {
		return
	}

	$.post(
		backendAddr+MESSAGES_PATH,
		msg,
		function(json) {
			$("#chat-box")
			json.msgs.forEach(m => {
				if (m !== "") {
					const origin = m.split(":@")[0];
					const msg = m.split(":@")[1];
					$("#chat-box").append(origin +" : "+ msg +"<br />");
				}
			})
		}
	)


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
		addr,
		function(json) {
			$("#node-box").empty()
			json.nodes.forEach(n => {
				$("#node-box").append(n+"<br />");
			})
 		}
	)

}

var getNewNode = function(){
	$.getJSON(
		backendAddr+NODE_PATH,
		function(json) {
			$("#node-box").empty()
			json.nodes.forEach(n => {
				$("#node-box").append(n+"<br />");
			})
 		}
	);

}

var getNewMsg = function(){
	// get new msgs
	$.get(
		"http://localhost:8080/message",
		function(json) {
			$("#chat-box")
			json.msgs.forEach(m => {
				if (m !== "") {
					const origin = m.split(":@")[0];
					const msg = m.split(":@")[1];
					$("#chat-box").append(origin +" : "+ msg +"<br />");
				}
			})
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
		}
	);

}

whenDocumentLoaded(() => {


	setInterval(getPeerId, 2*1000);

	setInterval(getNewNode, 2*1000);
	setInterval(getNewMsg, 2*1000);


	// Fetch the neihbors nodes add to be setup each
	$.getJSON(
		backendAddr+NODE_PATH,
		function(json) {
			$("#node-box").empty()
			json.nodes.forEach(n => {
				$("#node-box").append(n+"<br />");
			})
 		}
	);





	// Post new node
	let add = document.getElementById("add-btn");
	add.addEventListener("click", () => addPeer())


	// Post new messages
	let send = document.getElementById("send-btn");
	send.addEventListener("click", () => fetchAndSendMessage())






});



// Helpers functions

// from https://stackoverflow.com/questions/1527803/generating-random-whole-numbers-in-javascript-in-a-specific-range


/* 		const newContent = document.createTextNode(t);
		d.appendChild(newContent);
		container_element.appendChild(d); */
