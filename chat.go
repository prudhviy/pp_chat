package main

import (
    "time"
    
    "io"
    "io/ioutil"
    //"bytes"
    "net/http"
    "os"
    //"os/exec"
    
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
            $.ajax({
                type: 'POST',
                data: {'comm_id': comm_id},
                url: '/chat/join/',
                success: function(res){
                    $('.log').html(res);
                },
                complete: function(){
                    pp.chat.join(comm_id);
                },
                timeout: 8000
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
        <li>Log:<span class="log"></span></li>
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
    var err error
    
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Content-Type", "application/javascript")
    
    response := "message sent"

    recepientUserId := req.FormValue("comm_id")
    chatMessage := req.FormValue("msg")

    cEntity := users[recepientUserId]
    cEntity.recv <- chatMessage
    
    io.WriteString(w, response)
    if err == nil {
        fmt.Printf("Response sent- %s %s\n", req.URL, time.Now())
    }
}

func openPushChannel(comm_id string) (chan string) {
    fmt.Printf("\nJoin Chat > user-id:%v\n", comm_id)
    var newUser commEntity

    newUser.id = comm_id
    newUser.recv = make(chan string)
    users[newUser.id] = newUser

    return newUser.recv
}

func getChatMessage(recv chan string) (msg string) {

    timeout := time.After(12 * time.Second)
    select {
        case newMessage := <-recv:
            fmt.Printf("\ngot chat %s\n", newMessage)
            msg = newMessage
        case currentTime := <-timeout:
            msg = "timeout"
            fmt.Printf("\nServer timeout %s\n", currentTime)
    }

    return msg
}

func joinChat(w http.ResponseWriter, req *http.Request) {
    var err error
    
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Content-Type", "text/html")

    recv := openPushChannel(req.FormValue("comm_id"))
    newMessage := getChatMessage(recv)

    io.WriteString(w, newMessage)
    if err == nil {
        fmt.Printf("Response sent- %s %s\n", req.URL, time.Now())
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