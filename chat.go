package main

import (
    "time"
    
    "io"
    "io/ioutil"
    //"bytes"
    "net/http"
    "os"
    
    //"crypto/md5"
    //"hash"

    //"html/template"
    "log"
    "fmt"
    //"strings"
    //"encoding/json"
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
        pp.chat.join = function(comm_id) {
            var join_time = Math.round((new Date()).getTime() / 1000);
            $.ajax({
                type: 'POST',
                data: {'comm_id': comm_id, 'join_time': join_time},
                url: '/chat/join/',
                success: function(res){
                    var ele = '<li>' + res + '</li>'
                    $('.log').append($(ele));
                },
                complete: function(){
                    console.log('close conn', join_time);
                    //pp.chat.join(comm_id);
                },
                timeout: 60000
            });
        };
        pp.chat.send_msg = function(comm_id, msg) {
            $.ajax({
                type: 'POST',
                data: {'comm_id': comm_id, 'msg': msg},
                url: '/chat/message/',
                success: function(res){
                    console.log('msg sent');
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
    id string
    recv chan string
    //status string
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
    w.Header().Set("Content-Type", "application/javascript; charset=iso-8859-1")

    resourceData, err := ioutil.ReadFile("./jquery.js")
    if err != nil {
       fmt.Printf("###\nError - %s\n###\n", err)
       os.Exit(1)
    }

    io.WriteString(w, string(resourceData))
}

func sendMessage(w http.ResponseWriter, req *http.Request) {
    //var err error
    
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

func openPushChannel(comm_id string, join_time string) (chan string) {
    var newUser commEntity
    var userRecvChannel chan string

    _, exists := users[comm_id]
    if exists {
        fmt.Printf("Already joined> User-id:%v %v\n", comm_id, join_time)
        userRecvChannel = users[comm_id].recv
    } else {
        fmt.Printf("Join Chat> User-id:%v %v\n", comm_id, join_time)
        newUser.id = comm_id
        newUser.recv = make(chan string)
        users[newUser.id] = newUser
        userRecvChannel = newUser.recv
    }
    return userRecvChannel
}

func getChatMessage(recv chan string) (msg string) {
    timeout := time.After(5 * time.Second)
    select {
        case newMessage := <-recv:
            msg = newMessage
        case <-timeout:
            fmt.Printf("timeout yes\n")
            msg = "timeout"
    }
    return msg
}

func joinChat(w http.ResponseWriter, req *http.Request) {
    
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Content-Type", "text/html")

    fmt.Printf("JoinChat method> User-id:%s\n", req.FormValue("comm_id"))
    recv := openPushChannel(req.FormValue("comm_id"), req.FormValue("join_time"))
    newMessage := getChatMessage(recv)

    if newMessage == "timeout" {
        conn, _, _ := w.(http.Hijacker).Hijack()
        fmt.Printf("Timeout- close conn> User-id:%s %s %T\n", req.FormValue("comm_id"), req.FormValue("join_time"), conn)
        conn.Close()
        //response.Body.Close?
    } else {
        io.WriteString(w, newMessage)
        fmt.Printf("Chat sent> User-id:%s %s %s\n", req.FormValue("comm_id"), req.FormValue("join_time"), newMessage)
    }
}

func main() {
    // make sure app uses all cores
    flag.Parse()
    runtime.GOMAXPROCS(*numCores)

    fmt.Printf("\nChat server running at " +
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