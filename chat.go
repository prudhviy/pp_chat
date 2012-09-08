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
    <script type="text/javascript" src="./jquery.js"></script>
    <script type="text/javascript">
        $("#start_chat").live('click', function(event){
            $.ajax({
                type: 'POST',
                data: {'user_name': $('#user_name').val()},
                url: '/chat',
                success: function(res){
                    console.log('success');
                }
            });
        });
    </script>
</head>
<body>
    <form>
        <label>User name</label>
        <input id="user_name" type="text">
        <input id="start_chat" value="Start Chat"type="button">
    </form>
</body>
</html>
`

type Chann struct {
    out chan string
} 

var userChan map[string]Chann = make(map[string]Chann)

func testPage(w http.ResponseWriter, req *http.Request) {
    w.Header().Set("Server", "goclubby/0.1")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
    io.WriteString(w, testPageHTML)
}

func jquery(w http.ResponseWriter, req *http.Request) {
    w.Header().Set("Server", "goclubby/0.1")
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
    
    w.Header().Set("Server", "goclubby/0.1")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Content-Type", "application/javascript")
    response := "response 1"

    user_chan := new(Chann)
    out_chan := make(chan string)
    (*user_chan).out = out_chan
    userChan["user_a"] = *user_chan

    for resource := range out_chan {
        fmt.Printf("got chat %s", resource)
    }

    fmt.Printf("%s", req)
    io.WriteString(w, response)
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
    http.HandleFunc("/jquery.js", jquery)
    err := http.ListenAndServe("0.0.0.0:8000", nil)
    if err != nil {
        log.Fatal("In main(): ", err)
    }
}