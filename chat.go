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
                    console.log('success');
                }
            });
        };
        $("#join_chat").live('click', function(event){
            pp.chat.join($("#user_id").val());
        });
    </script>
</head>
<body>
    <form>
        <label>User ID:</label>
        <input id="user_id" type="text">
        <input id="join_chat" value="Join Chat!" type="button">
    </form>
    <span>Online Users</span>
    <div>
    %s
    </div>
</body>
</html>
`

type commEntity struct {
    id string
    recv chan string
    //status string
}

//var room map[string]Chann = make(map[string]Chann)

var users []commEntity = make([]commEntity, 0)

func getTestData() ([]commEntity) {
    return users
}

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

func chat(w http.ResponseWriter, req *http.Request) {
    var err error
    
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Content-Type", "application/javascript")
    
    response := "response 1"
    
    io.WriteString(w, response)
    if err == nil {
        fmt.Printf("Response sent- %s %s\n", req.URL, time.Now())
    }
}

func openPushChannel(comm_id string) {
    fmt.Printf("###\nJoin Chat > user-id:%v\n###\n", comm_id)
    var newUser commEntity

    newUser.id = comm_id
    newUser.recv = make(chan string)
    users = append(users, newUser)
    
    for resource := range newUser.recv {
        fmt.Printf("got chat %s", resource)
    }
}


func joinChat(w http.ResponseWriter, req *http.Request) {
    var err error
    
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Content-Type", "text/html")

    fmt.Printf("###\nList of online Users > user-id:%v\n###\n", users)
    openPushChannel(req.FormValue("comm_id"))

    io.WriteString(w, "joined")
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
    http.HandleFunc("/chat", chat)
    http.HandleFunc("/chat/join/", joinChat)
    http.HandleFunc("/jquery.js", jquery)
    err := http.ListenAndServe("0.0.0.0:8000", nil)
    if err != nil {
        log.Fatal("In main(): ", err)
    }
}