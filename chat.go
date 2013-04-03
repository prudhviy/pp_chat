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
	"sync"

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

type TimestampedMessage struct {
	CreatedTime int64
	Value interface{}
	Type string
}

type commEntity struct {
	id   string
	recv chan TimestampedMessage
	groupId string
	//status string
	lastActiveSince int64
}

type ConcurrentUsersMap struct {
	mu sync.RWMutex
	m map[string]commEntity
}

func (u ConcurrentUsersMap) Get(comm_id string) commEntity {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.m[comm_id]
}

func (u ConcurrentUsersMap) Set(comm_id string, value commEntity) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.m[comm_id] = value
}

func (u ConcurrentUsersMap) Contains(comm_id string) (exists bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	_, exists = u.m[comm_id]
	return exists
}

func NewUsers() ConcurrentUsersMap {
	return ConcurrentUsersMap{m: make(map[string]commEntity)}
}

var users = NewUsers()

//var users map[string]commEntity = make(map[string]commEntity)

var numCores = flag.Int("n", runtime.NumCPU(), "number of CPU cores to use")

var testPageHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Chat Room</title>
    <script type="text/javascript" src="./jquery.js"></script>
    <script type="text/javascript">
        var pp = pp || {};
        pp.presence = pp.presence || {};
        pp.presence.domain = "";
        pp.counter = 0;
        pp.presence.join = function(comm_id) {
            var join_time = Math.round((new Date()).getTime() / 1000);
            $.ajax({
                type: 'POST',
                dataType: 'json',
                data: {'comm_id': comm_id, 'join_time': join_time, 'group_id': 123},
                url: pp.presence.domain + '/subscribe/message/',
                success: function(res){
                    console.log(typeof res);
                    console.log(res);
                    var ele = '<li>' + res.Value + '&nbsp;|&nbsp;' + res.Type +'</li>'
                    $('.log').append($(ele));
                },
                complete: function(){
                    //console.log('close conn', join_time);
                    var stri = "pp.presence.join(" + comm_id + ")";
                    setTimeout(stri, 100);
                },
                timeout: 20000
            });
        };
        pp.presence.send_msg = function(comm_id, msg) {
        	pp.counter = pp.counter + 1;
            console.log(pp.counter);
        	msg = msg + " " + pp.counter
            $.ajax({
                type: 'POST',
                data: {'comm_id': comm_id, 'msg': msg},
                url: pp.presence.domain + '/send/message/',
                success: function(res){
                	var x = 1;
                },
                timeout: 5000
            });
        };
        pp.presence.getAllOnlineUsers = function(comm_id) {
        	$.ajax({
                type: 'POST',
                dataType: 'text/html',
                data: {'comm_id': comm_id, 'group_id': 123},
                url: pp.presence.domain + '/get/onlineUsers/',
                success: function(res){
                    console.log(res);
                },
                timeout: 20000
            });
        };
        $("#join_chat").live('click', function(event){
            pp.presence.join($("#user_id").val());
            pp.presence.getAllOnlineUsers($("#user_id").val());
        });
        $("#send_msg").live('click', function(event){
            pp.presence.send_msg($("#recv_id").val(), $("#recv_msg").val());
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

//var room map[string]Chann = make(map[string]Chann)

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


func fetchAllOnlineUsers(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	go getAllOnlineUsers(req.FormValue("comm_id"), req.FormValue("group_id"))

	io.WriteString(w, "success")
}

func userActive(lastActiveSince int64) (active bool) {
	currentUnixTime := (time.Now()).Unix()
	active = false
	if currentUnixTime - lastActiveSince < 4 {
		active = true
	}
	return
}

func messageActive(createdTime int64) (active bool) {
	currentUnixTime := (time.Now()).Unix()
	active = false
	if currentUnixTime - createdTime < 15 {
		active = true
	}
	return
}

func getAllOnlineUsers(requestingCommId string, group_id string) {
	var message TimestampedMessage
	var onlineUsers []string
	
	users.mu.RLock()
	for _, userCommEntity := range users.m {
		if userCommEntity.groupId == group_id && userCommEntity.id != requestingCommId && userActive(userCommEntity.lastActiveSince) {
			onlineUsers = append(onlineUsers, userCommEntity.id)
		}
	}
	users.mu.RUnlock()
	exists := users.Contains(requestingCommId)
	if exists {
		requestingUser := users.Get(requestingCommId)
		message.Value = onlineUsers
		message.CreatedTime = (time.Now()).Unix()
		message.Type = "allpresence"
		requestingUser.recv <- message
	}
}

func sendMessage(w http.ResponseWriter, req *http.Request) {
	//var err error
	var message TimestampedMessage
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "application/javascript")

	response := "message sent"

	recepientUserId := req.FormValue("comm_id")
	message.Value = req.FormValue("msg")
	message.CreatedTime = (time.Now()).Unix()
	message.Type = "chat"

	cEntity := users.Get(recepientUserId)
	if userActive(cEntity.lastActiveSince) {
		cEntity.recv <- message
	}

	io.WriteString(w, response)
	/*if err == nil {
	    fmt.Printf("Response sent- %s %s\n", req.URL, time.Now())
	}*/
}

func openPushChannel(comm_id string, group_id string, join_time string) chan TimestampedMessage {
	var newUser commEntity
	var tempUser commEntity
	var userRecvChannel chan TimestampedMessage
	//var tempTime time.Time

	currentUnixTime := (time.Now()).Unix()
	
	exists := users.Contains(comm_id)
	if exists {
		fmt.Printf("Already joined> User-id:%v %v\n", comm_id, join_time)
		// work around for bug http://code.google.com/p/go/issues/detail?id=3117
		tempUser = users.Get(comm_id)
		tempUser.lastActiveSince = currentUnixTime
		users.Set(comm_id, tempUser)
		tempoUser := users.Get(comm_id)
		userRecvChannel = tempoUser.recv

	} else {
		fmt.Printf("New join> User-id:%v %v\n", comm_id, join_time)
		newUser.id = comm_id
		newUser.recv = make(chan TimestampedMessage, 100)
		newUser.lastActiveSince = currentUnixTime
		newUser.groupId = group_id
		
		users.Set(newUser.id, newUser)
		userRecvChannel = newUser.recv
		// notify all users of the group about the new user
		go notifyNewUserToGroup(comm_id, group_id)
	}
	return userRecvChannel
}

func notifyNewUserToGroup(comm_id string, group_id string) {
	var message TimestampedMessage
	var newUser []string = []string{comm_id}
	users.mu.RLock()
	defer users.mu.RUnlock()
	for _, userCommEntity := range users.m {
		if userCommEntity.groupId == group_id && userCommEntity.id != comm_id && userActive(userCommEntity.lastActiveSince) {
			message.Value = newUser
			message.CreatedTime = (time.Now()).Unix()
			message.Type = "presence"
			userCommEntity.recv <- message
		}
	}
}

func notifyUserOfflineToGroup(comm_id string, group_id string) {
	var message TimestampedMessage
	var newUser []string = []string{comm_id}
	
	timeout := time.After(2 * time.Second)
	select {
	case <-timeout:
		userCommEntity := users.Get(comm_id)
		if !userActive(userCommEntity.lastActiveSince) {
			users.mu.RLock()
			defer users.mu.RUnlock()
			for _, userCommEntity := range users.m {
				if userCommEntity.groupId == group_id && userCommEntity.id != comm_id {
					message.Value = newUser
					message.CreatedTime = (time.Now()).Unix()
					message.Type = "offpresence"
					userCommEntity.recv <- message
				}
			}
		}
	}
}

func getMessage(recv chan TimestampedMessage) (msg TimestampedMessage) {
	timeout := time.After(60 * time.Second)
	select {
		case newMessage := <-recv:
			msg = newMessage
		case <-timeout:
			msg.CreatedTime = (time.Now()).Unix()
			msg.Value = "serverTimeout"
			msg.Type = "serverTimeout"
	}
	return msg
}

func subscribeMessage(w http.ResponseWriter, req *http.Request) {
	recv := openPushChannel(req.FormValue("comm_id"), req.FormValue("group_id"), req.FormValue("join_time"))

	hjConn, bufrw := httpHijack(w)
	defer hjConn.Close()
	go notifyClientDisconnect(bufrw, recv, req.FormValue("comm_id"), req.FormValue("group_id"))
	newMessage := getMessage(recv)

	buildHTTPResponse(bufrw)

	if newMessage.Type == "serverTimeout" {
		fmt.Printf("Server timed out> User-id:%s %s \n",
					req.FormValue("comm_id"),
					req.FormValue("join_time"))
	} else if newMessage.Type == "clientClose" {
		fmt.Printf("Client closed conn> User-id:%s %s \n",
					req.FormValue("comm_id"),
					req.FormValue("join_time"))
	} else if newMessage.Type == "presence" {
		fmt.Printf("Presence Message sent to> User-id:%s %s %s\n", req.FormValue("comm_id"),
						req.FormValue("join_time"), newMessage.Value)
	} else if newMessage.Type == "chat" {
		fmt.Printf("Message sent to> User-id:%s %s %s\n", req.FormValue("comm_id"),
						req.FormValue("join_time"), newMessage.Value)
	}
	if messageActive(newMessage.CreatedTime) {
		marshalData, _ := json.Marshal(newMessage)
		jsonResponse := string(marshalData)
		fmt.Printf("json: %s \n", jsonResponse)
		// write body 
		bufrw.WriteString(jsonResponse + "\r\n")
		bufrw.Flush()
	}
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

func notifyClientDisconnect(bufrw *bufio.ReadWriter, recv chan TimestampedMessage, comm_id string, group_id string) {
	var message TimestampedMessage

	// listen if client has closed the connection
	bs, err := bufrw.Reader.Peek(1)
	if len(bs) == 0 && err != nil {
		//fmt.Printf("error: %v %T %#v\n", err, err, err)
		if _, ok := err.(*net.OpError); ok {
			fmt.Printf("server side hjConn close\n")
		} else {
			//fmt.Printf("client side hjConn close\n")
			message.CreatedTime = (time.Now()).Unix()
			message.Value = "clientClose"
			message.Type = "clientClose"
			recv <- message
		}
		go notifyUserOfflineToGroup(comm_id, group_id)
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
	fmt.Printf("\nPresence server running at "+
		"http://0.0.0.0:8000 on %d CPU cores\n", *numCores)
	http.HandleFunc("/", testPage)
	http.HandleFunc("/jquery.js", jquery)
	http.HandleFunc("/subscribe/message/", subscribeMessage)
	http.HandleFunc("/send/message/", sendMessage)
	http.HandleFunc("/get/onlineUsers/", fetchAllOnlineUsers)

	err := http.ListenAndServe("0.0.0.0:8000", nil)
	if err != nil {
		log.Fatal("In main(): ", err)
	}
}
