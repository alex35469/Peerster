
/*
	Run the action when we are sure the DOM has been loaded
*/
const backendAddr = "http://127.0.0.1:8080";
const ID_PATH = "/id";
const MESSAGES_PATH = "/message"
const NODE_PATH = "/node"


function whenDocumentLoaded(action) {
	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", action);
	} else {
		// `DOMContentLoaded` already fired

		action();
	}
}




function fetchAndSendMessage(){
	let msg = document.getElementsByName("send-input")[0].value;
	if (msg === "") {
		return
	}


	console.log(msg)

}

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

/* 	$.ajax({type: "POST", url: "http://localhost:8080/", success: function(result){
			$("#div1").html(result);
	}}); */

}



whenDocumentLoaded(() => {


	(function() {$.getJSON(
		"http://127.0.0.1:8080/id",
		function(json) {
			console.log( "JSON Data: " + json.ID );
 		}
	);})()

	//document.getElementById("peerID").innerText = Name + " - " + Peer_ID

	// Load jQuery
	const script = document.createElement('script');
	script.src = 'http://code.jquery.com/jquery-1.11.0.min.js';
	script.type = 'text/javascript';
	document.getElementsByTagName('head')[0].appendChild(script);

	let send = document.getElementById("send-btn");

	send.addEventListener("click", () => fetchAndSendMessage())

	let add = document.getElementById("add-btn");
	add.addEventListener("click", () => addPeer())



});



// Helpers functions

// from https://stackoverflow.com/questions/1527803/generating-random-whole-numbers-in-javascript-in-a-specific-range


/* 		const newContent = document.createTextNode(t);
		d.appendChild(newContent);
		container_element.appendChild(d); */
