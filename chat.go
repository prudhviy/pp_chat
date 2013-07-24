package main

import (
	"time"

	"bufio"
	"io"
	"io/ioutil"
	//"bytes"
	"net"
	"net/http"
	"os"
	"sync"

	//"crypto/md5"
	//"hash"

	//"html/template"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	//"encoding/hex"

	"flag"
	"runtime"
	//"reflect"
)

const serverTimeout int64 = 45
const allowedOnOffDiff int64 = 3

type TimestampedMessage struct {
	Type        string
	Value       interface{}
	CreatedTime int64
}

type commEntity struct {
	id           string
	recv         chan TimestampedMessage
	group_id     string
	status       string
	onlineSince  int64
	offlineSince int64
	onOffDiff    int64
}

type ConcurrentUsersMap struct {
	mu sync.RWMutex
	m  map[string]commEntity
}

func (u ConcurrentUsersMap) Get(comm_id string) commEntity {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.m[comm_id]
}

func (u ConcurrentUsersMap) GetUserCommEntities(prefix_id string) (userCommEntities []commEntity) {
	prefix_id = prefix_id + "_"
	u.mu.RLock()
	defer u.mu.RUnlock()
	for _, userCommEntity := range u.m {
		if strings.Contains(userCommEntity.id, prefix_id) {
			userCommEntities = append(userCommEntities, userCommEntity)
		}
	}
	return
}

func (u ConcurrentUsersMap) GetAllGroupUsers(group_id string) (groupCommEntities []commEntity) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	for _, userCommEntity := range u.m {
		if userCommEntity.group_id == group_id {
			groupCommEntities = append(groupCommEntities, userCommEntity)
		}
	}
	return
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

//var room map[string]Chann = make(map[string]Chann)

func testPage(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")

	resourceData, err := ioutil.ReadFile("./testPage.html")
	if err != nil {
		fmt.Printf("###\nError - %s\n###\n", err)
		os.Exit(1)
	}

	io.WriteString(w, string(resourceData))
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

func cssStyle(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/css; charset=utf-8")

	resourceData, err := ioutil.ReadFile("./style.css")
	if err != nil {
		fmt.Printf("###\nError - %s\n###\n", err)
		os.Exit(1)
	}

	io.WriteString(w, string(resourceData))
}

func writeMessageUser(comm_id string, message TimestampedMessage) {
	recepientUser := users.Get(comm_id)
	if userActive(recepientUser) {
		recepientUser.recv <- message
	}
	// else - write to DB ?
}

func getAllOnlineUsers(requestingCommId string, group_id string) {
	var message TimestampedMessage
	var onlineUsers []string
	prefix_clientId := strings.Split(requestingCommId, "_")
	prefix_id := prefix_clientId[0] + "_"
	exists := users.Contains(requestingCommId)
	if exists {
		groupCommEntities := users.GetAllGroupUsers(group_id)
		for _, cEntity := range groupCommEntities {
			if userActive(cEntity) {
				if !strings.Contains(cEntity.id, prefix_id) {
					onlineUsers = append(onlineUsers, cEntity.id)
				}
			}
		}
		message.Value = onlineUsers
		message.CreatedTime = (time.Now()).Unix()
		message.Type = "allpresence"
		writeMessageUser(requestingCommId, message)
	}
}

func fetchAllOnlineUsers(w http.ResponseWriter, req *http.Request) {
	var message TimestampedMessage
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	go getAllOnlineUsers(req.FormValue("comm_id"), req.FormValue("group_id"))

	message.CreatedTime = (time.Now()).Unix()
	message.Value = "success"
	message.Type = "success"
	marshalData, _ := json.Marshal(message)
	jsonResponse := string(marshalData)
	io.WriteString(w, jsonResponse)
}

func userActive(cEntity commEntity) (active bool) {
	currentUnixTime := (time.Now()).Unix()
	active = false
	users.mu.Lock()
	defer users.mu.Unlock()
	if cEntity.status == "active" {
		active = true
	} else {
		onOffdiff := cEntity.onOffDiff - (2 * cEntity.onOffDiff)
		currOffDiff := currentUnixTime - cEntity.offlineSince
		// when status is 'inactive', onOffDiff should be between 0 & serverTimeout
		// Reason: since its inactive, last event is offline.
		if 0 <= onOffdiff && onOffdiff <= serverTimeout {
			// currOffDiff should be < 3. Reason: two cases -
			// 1. conn is closed and client is connecting back(within few seconds gap)
			// 2. conn is closed and client is not connecting back at all
			// if its connecting back it will definetely connect back in less than 3
			if currOffDiff < allowedOnOffDiff {
				active = true
			}
		}
		fmt.Printf("Online -- %v %s = %d = %d\n", active, cEntity.id, onOffdiff, currOffDiff)
	}
	//fmt.Printf("Online status %v '%s'\n", active, cEntity.status)
	return
}

func messageActive(createdTime int64) (active bool) {
	currentUnixTime := (time.Now()).Unix()
	active = false
	if currentUnixTime-createdTime < 15 {
		active = true
	}
	return
}

func sendMessage(w http.ResponseWriter, req *http.Request) {
	//var err error
	var message TimestampedMessage
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "application/javascript")

	response := "message sent"

	comm_id_prefix := req.FormValue("comm_id")
	group_id := req.FormValue("group_id")
	message.Value = req.FormValue("msg")
	message.CreatedTime = (time.Now()).Unix()
	message.Type = "chat"
	cEntities := users.GetUserCommEntities(comm_id_prefix)
	//fmt.Printf("===== %v \n%T \n%#v =====\n", cEntities, cEntities, cEntities)
	for _, cEntity := range cEntities {
		if cEntity.group_id == group_id {
			writeMessageUser(cEntity.id, message)
		}
	}

	io.WriteString(w, response)
	/*if err == nil {
	    fmt.Printf("Response sent- %s %s\n", req.URL, time.Now())
	}*/
}

func openPushChannel(comm_id string, group_id string) chan TimestampedMessage {
	var newUser commEntity
	var userRecvChannel chan TimestampedMessage
	//var tempTime time.Time

	currentUnixTime := (time.Now()).Unix()

	exists := users.Contains(comm_id)
	if exists {
		//fmt.Printf("Already joined> User-id:%v \n", comm_id)
		// work around for bug http://code.google.com/p/go/issues/detail?id=3117
		tempUser := users.Get(comm_id)
		tempUser.onlineSince = currentUnixTime
		tempUser.onOffDiff = tempUser.onlineSince - tempUser.offlineSince
		tempUser.status = "active"
		users.Set(comm_id, tempUser)
		tempoUser := users.Get(comm_id)
		userRecvChannel = tempoUser.recv
		if tempUser.onOffDiff > allowedOnOffDiff {
			go notifyUserActiveToGroup(comm_id, group_id)
		}
	} else {
		fmt.Printf("New join> User-id:%v \n", comm_id)
		newUser.id = comm_id
		newUser.recv = make(chan TimestampedMessage, 100)
		newUser.onlineSince = currentUnixTime
		newUser.offlineSince = currentUnixTime - 3
		newUser.onOffDiff = newUser.onlineSince - newUser.offlineSince
		newUser.status = "active"
		newUser.group_id = group_id

		users.Set(newUser.id, newUser)
		userRecvChannel = newUser.recv
		// notify all users of the group about the new user
		go notifyUserActiveToGroup(comm_id, group_id)
	}
	return userRecvChannel
}

func notifyUserActiveToGroup(comm_id string, group_id string) {
	var message TimestampedMessage
	var newUser []string = []string{comm_id}
	prefix_clientId := strings.Split(comm_id, "_")
	prefix_id := prefix_clientId[0] + "_"
	groupCommEntities := users.GetAllGroupUsers(group_id)
	for _, cEntity := range groupCommEntities {
		if !strings.Contains(cEntity.id, prefix_id) {
			message.Value = newUser
			message.CreatedTime = (time.Now()).Unix()
			message.Type = "presence"
			writeMessageUser(cEntity.id, message)
		}
	}
}

func notifyUserOfflineToGroup(comm_id string, group_id string) {
	var message TimestampedMessage
	var offlineUser []string = []string{comm_id}
	prefix_clientId := strings.Split(comm_id, "_")
	prefix_id := prefix_clientId[0]
	timeout := time.After(3 * time.Second)
	select {
	case <-timeout:
		cUserEntities := users.GetUserCommEntities(prefix_id)
		active := false
		for _, cUserEntity := range cUserEntities {
			if userActive(cUserEntity) {
				active = true
				break
			}
		}
		//fmt.Printf("@User %s is %v %v\n", prefix_id, active, cUserEntities)
		if !active {
			groupCommEntities := users.GetAllGroupUsers(group_id)
			for _, cEntity := range groupCommEntities {
				if cEntity.id != comm_id {
					message.Value = offlineUser
					message.CreatedTime = (time.Now()).Unix()
					message.Type = "offpresence"
					writeMessageUser(cEntity.id, message)
				}
			}
		}
	}
}

func getMessage(recv chan TimestampedMessage, hjConnChan chan TimestampedMessage) (msg TimestampedMessage) {
	timeout := time.After(45 * time.Second)
	select {
	case newMessage := <-recv:
		msg = newMessage
	case <-timeout:
		msg.CreatedTime = (time.Now()).Unix()
		msg.Value = "serverTimeout"
		msg.Type = "serverTimeout"
	case <-hjConnChan:
		msg.CreatedTime = (time.Now()).Unix()
		msg.Value = "clientClose"
		msg.Type = "clientClose"
	}
	return msg
}

func setCommEntityInactive(comm_id string) {
	cEntity := users.Get(comm_id)
	cEntity.status = "inactive"
	cEntity.offlineSince = (time.Now()).Unix()
	cEntity.onOffDiff = cEntity.onlineSince - cEntity.offlineSince
	users.Set(comm_id, cEntity)
}

func subscribeMessage(w http.ResponseWriter, req *http.Request) {
	var hjConnChan chan TimestampedMessage = make(chan TimestampedMessage, 1)

	recv := openPushChannel(req.FormValue("comm_id"), req.FormValue("group_id"))
	hjConn, bufrw := httpHijack(w)
	defer hjConn.Close()
	go notifyClientDisconnect(bufrw, hjConnChan, req.FormValue("comm_id"), req.FormValue("group_id"), req.FormValue("join_time"))
	newMessage := getMessage(recv, hjConnChan)

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
		fmt.Printf("\tjson: %s \n", jsonResponse)
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

func notifyClientDisconnect(bufrw *bufio.ReadWriter, hjConnChan chan TimestampedMessage, comm_id string, group_id string, join_time string) {
	var message TimestampedMessage

	// listen if client has closed the connection
	bs, err := bufrw.Reader.Peek(1)
	if len(bs) == 0 && err != nil {
		//fmt.Printf("error: %v %T %#v\n", err, err, err)
		if _, ok := err.(*net.OpError); ok {
			fmt.Printf("Server closed conn> User-id:%s %s\n", comm_id, join_time)
		} else {
			//fmt.Printf("client side hjConn close\n")
			message.CreatedTime = (time.Now()).Unix()
			message.Value = "clientClose"
			message.Type = "clientClose"
			hjConnChan <- message
		}
		setCommEntityInactive(comm_id)
		notifyUserOfflineToGroup(comm_id, group_id)
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
		"http://127.0.0.1:8088 on %d CPU cores\n", *numCores)
	http.HandleFunc("/", testPage)
	http.HandleFunc("/jquery.js", jquery)
	http.HandleFunc("/style.css", cssStyle)
	http.HandleFunc("/subscribe/message/", subscribeMessage)
	http.HandleFunc("/send/message/", sendMessage)
	http.HandleFunc("/get/onlineUsers/", fetchAllOnlineUsers)

	err := http.ListenAndServe("127.0.0.1:8088", nil)
	if err != nil {
		log.Fatal("In main(): ", err)
	}
}
