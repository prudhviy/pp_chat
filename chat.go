package main

import (
	"time"

	"io"
	"io/ioutil"
	"bufio"
	//"bytes"
	"net"
	"net/http"
	"os"

	//"crypto/md5"
	//"hash"

	//"html/template"
	"log"
	"fmt"
	//"strings"
	"encoding/json"
	//"encoding/hex"

	"runtime"
	"flag"
	//"reflect"
)

var numCores = flag.Int("n", runtime.NumCPU(), "number of CPU cores to use")

var testPageHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Chat Room</title>
    <script type="text/javascript" src="./jquery.js"></script>
    <script type="text/javascript">
        var pp = pp || {};
        pp.chat = pp.chat || {};
        pp.chat.domain = ""
        pp.chat.join = function(comm_id) {
            var join_time = Math.round((new Date()).getTime() / 1000);
            $.ajax({
                type: 'POST',
                dataType: 'json',
                data: {'comm_id': comm_id, 'join_time': join_time, 'group_id': 123},
                url: pp.chat.domain + '/chat/join/',
                success: function(res){
                    console.log(typeof res);
                    console.log(res);
                    var ele = '<li>' + res.OnlineUsers + '</li>'
                    $('.log').append($(ele));
                },
                complete: function(){
                    //console.log('close conn', join_time);
                    pp.chat.join(comm_id);
                },
                timeout: 20000
            });
        };
        pp.chat.send_msg = function(comm_id, msg) {
            $.ajax({
                type: 'POST',
                data: {'comm_id': comm_id, 'msg': msg},
                url: pp.chat.domain + '/chat/message/',
                timeout: 5000,
                success: function(res){
                	var x = 1;
                    //console.log('msg sent');
                }
            });
        };
        $("#join_chat").live('click', function(event){
            pp.chat.join($("#user_id").val());
        });
        $("#send_msg").live('click', function(event){
            pp.chat.send_msg($("#recv_id").val(), $("#recv_msg").val());
        });
    </script>
</head>
<body>
    <form>
        <ul>
        <li>
            <label>User ID:</label>
            <input id="user_id" type="text">
            <input id="join_chat" value="Join Chat!" type="button">
        </li>
        <li>
            <label>Recepient User ID:</label>
            <input id="recv_id" type="text">
            <label>Message:</label>
            <input id="recv_msg" type="text">
            <input id="send_msg" value="Send Message!" type="button">
        </li>
        </ul>
        <div>Log:<ul class="log"></ul></div>
    </form>
</body>
</html>
`

type commEntity struct {
	id   string
	recv chan string
	groupId string
	//status string
	lastActiveSince int64
}

type PresenceMessage struct {
    OnlineUsers string
}

//var room map[string]Chann = make(map[string]Chann)

var users map[string]commEntity = make(map[string]commEntity)

/*func getTestData() ([string]commEntity) {
    return users
}*/

func testPage(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")

	io.WriteString(w, testPageHTML)
}

func jquery(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type",
		"application/javascript; charset=iso-8859-1")

	resourceData, err := ioutil.ReadFile("./jquery.js")
	if err != nil {
		fmt.Printf("###\nError - %s\n###\n", err)
		os.Exit(1)
	}

	io.WriteString(w, string(resourceData))
}

func sendMessage(w http.ResponseWriter, req *http.Request) {
	//var err error

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "application/javascript")

	response := "message sent"

	recepientUserId := req.FormValue("comm_id")
	chatMessage := req.FormValue("msg")

	cEntity := users[recepientUserId]
	cEntity.recv <- chatMessage

	io.WriteString(w, response)
	/*if err == nil {
	    fmt.Printf("Response sent- %s %s\n", req.URL, time.Now())
	}*/
}

func openPushChannel(comm_id string, group_id string, join_time string) chan string {
	var newUser commEntity
	var tempUser commEntity
	var userRecvChannel chan string
	var tempTime time.Time

	tempTime = time.Now()
	currentUnixTime := tempTime.Unix()

	_, exists := users[comm_id]
	if exists {
		fmt.Printf("Already joined> User-id:%v %v\n", comm_id, join_time)
		// work around for bug http://code.google.com/p/go/issues/detail?id=3117
		tempUser = users[comm_id]
		tempUser.lastActiveSince = currentUnixTime
		users[comm_id] = tempUser
		userRecvChannel = users[comm_id].recv

	} else {
		fmt.Printf("Join Chat> User-id:%v %v\n", comm_id, join_time)
		newUser.id = comm_id
		newUser.recv = make(chan string, 100)
		newUser.lastActiveSince = currentUnixTime
		newUser.groupId = group_id
		
		users[newUser.id] = newUser
		userRecvChannel = newUser.recv
		// notify all users of the group about the new user
		go notifyNewUserToGroup(comm_id, group_id)
	}
	return userRecvChannel
}

func notifyNewUserToGroup(comm_id string, group_id string) {
	for _, userCommEntity := range users {
		if userCommEntity.groupId == group_id && userCommEntity.id != comm_id {
			userCommEntity.recv <- comm_id
		}
	}
}

func getChatMessage(recv chan string) (msg string) {
	timeout := time.After(60 * time.Second)
	select {
		case newMessage := <-recv:
			msg = newMessage
		case <-timeout:
			msg = "timeout"
	}
	return msg
}

func joinChat(w http.ResponseWriter, req *http.Request) {
	var pMessage PresenceMessage
	
	recv := openPushChannel(req.FormValue("comm_id"), req.FormValue("group_id"), req.FormValue("join_time"))

	hjConn, bufrw := httpHijack(w)
	defer hjConn.Close()
	go notifyClientDisconnect(bufrw, recv)
	newMessage := getChatMessage(recv)

	buildHTTPResponse(bufrw)

	if newMessage == "timeout" {
		fmt.Printf("server timed out\n")
	} else if newMessage == "client_closed" {
		fmt.Printf("client closed conn> User-id:%s %s \n",
					req.FormValue("comm_id"),
					req.FormValue("join_time"))
	} else {
		pMessage.OnlineUsers = newMessage
		marshalData, _ := json.Marshal(pMessage)
		jsonResponse := string(marshalData)
		fmt.Printf("json: %s \n", jsonResponse)
		
		// write body 
		bufrw.WriteString(jsonResponse + "\r\n")
		fmt.Printf("Chat sent to> User-id:%s %s %s\n", req.FormValue("comm_id"),
						req.FormValue("join_time"), newMessage)
	}

	bufrw.Flush()
}

func buildHTTPResponse(bufrw *bufio.ReadWriter) {

	var newHeader http.Header = make(http.Header)

	newHeader.Add("Access-Control-Allow-Origin", "*")
	newHeader.Add("Content-Type", "application/json; charset=UTF-8")
	newHeader.Add("Cache-Control", "no-cache")
	newHeader.Add("X-AppServer", "GoAPP")
	// write status line
	bufrw.WriteString("HTTP/1.1 200 OK" + "\r\n")
	// write headers
	_ = newHeader.Write(bufrw)
	// write a black line
	bufrw.WriteString("\n")

}

func notifyClientDisconnect(bufrw *bufio.ReadWriter, recv chan string) {
	// listen if client has closed the connection
	bs, err := bufrw.Reader.Peek(1)
	if len(bs) == 0 && err != nil {
		//fmt.Printf("error: %v %T %#v\n", err, err, err)
		if _, ok := err.(*net.OpError); ok {
			fmt.Printf("server side hjConn close\n")
		} else {
			//fmt.Printf("client side hjConn close\n")
			recv <- "client_closed"
		}
	}
}

func httpHijack(w http.ResponseWriter) (net.Conn, *bufio.ReadWriter) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking",
			http.StatusInternalServerError)
	}
	// hijack http connection to tcp
	hjConn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	return hjConn, bufrw
}


func main() {
	// make sure app uses all cores
	flag.Parse()
	runtime.GOMAXPROCS(*numCores)
	fmt.Printf("\nChat server running at "+
		"http://0.0.0.0:8000 on %d CPU cores\n", *numCores)
	http.HandleFunc("/", testPage)
	http.HandleFunc("/jquery.js", jquery)
	http.HandleFunc("/chat/join/", joinChat)
	http.HandleFunc("/chat/message/", sendMessage)

	err := http.ListenAndServe("0.0.0.0:8000", nil)
	if err != nil {
		log.Fatal("In main(): ", err)
	}
}
