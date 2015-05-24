package main

import (
    "fmt"
    "os"
    "strconv"
    "errors"
    "runtime"
    "net/http"
    "strings"
    "io"
)

const VERSION = "1.0.0"

func goSharpParseOptions() (port, nbCpus int, pports string, err error) {
    fmt.Println("go-sharp", VERSION, "is a go srs http-flv advanced reverse proxy, auto detect and load balance")

    if len(os.Args) <= 3 {
        fmt.Println("Usage:", os.Args[0], "<port> <nb_cpus> <proxy_port0>[,<proxy_port1>,<proxy_portN>]")
        fmt.Println("       port: The port to listen.")
        fmt.Println("       nb_cpus: The number of cpu to use.")
        fmt.Println("       proxy_port0-N: The local port to proxy.")
        fmt.Println("For example:")
        fmt.Println("   ", os.Args[0], 8088, 1, 8080)
        fmt.Println("   ", os.Args[0], 8088, 1, "8080,8081,8082")
        os.Exit(-1)
    }

    if port, err = strconv.Atoi(os.Args[1]); err != nil {
        err = errors.New(fmt.Sprintf("invalid port %v and error is %s", os.Args[1], err.Error()))
        return
    }
    //fmt.Println("listen port", port)

    if nbCpus, err = strconv.Atoi(os.Args[2]); err != nil {
        err = errors.New(fmt.Sprintf("invalid nb_cpus %v and error is %s", os.Args[2], err.Error()))
        return
    }
    //fmt.Println("nb_cpus is", nbCpus)

    pports = os.Args[3]
    //fmt.Println("pports is", pports)

    return
}

type GoSharpNode struct {
    ID string
    URL string
    Load int64
}
func (v *GoSharpNode) DoProxy(w http.ResponseWriter, req *http.Request) {
    v.Load++

    proxy_url := fmt.Sprintf("http://127.0.0.1:%v%v", v.URL, req.RequestURI)
    fmt.Println(fmt.Sprintf("serve %v, proxy http://%v%v to %v", req.RemoteAddr, req.Host, req.RequestURI, proxy_url))

    v.doProxy(proxy_url, w, req)
    v.Load--
}
func (v *GoSharpNode) doProxy(url string, w http.ResponseWriter, req *http.Request) {
    if proxy_req,err := http.Get(url); err != nil {
        fmt.Println(fmt.Sprintf("serve %v, proxy failed, err is %v", req.RemoteAddr, err))
        return
    } else {
        written,_ := io.Copy(w, proxy_req.Body)
        fmt.Println(fmt.Sprintf("server %v, proxy completed, written is %v", req.RemoteAddr, written))
    }
}

type GoSharpContext struct {
    // key: id of node
    LB map[string]*GoSharpNode
}
func NewGoSharpContext(servers []string) *GoSharpContext {
    v := &GoSharpContext{}

    v.LB = make(map[string]*GoSharpNode)

    for _,server := range servers {
        node := &GoSharpNode{}
        node.ID = server
        node.URL = server
        v.LB[node.ID] = node
    }

    return v
}
func (v *GoSharpContext) ChooseBest() *GoSharpNode {
    var match *GoSharpNode

    // TODO: FIXME: support detect node status.
    for _,node := range v.LB {
        if match == nil || match.Load > node.Load {
            match = node
        }
    }

    return match
}

func goSharpRun() int {
    port, nbCpus, pports, err := goSharpParseOptions()
    if err != nil {
        fmt.Println(err)
        return -1
    }

    // the target SRS to proxy to
    targets := strings.Split(pports, ",")
    fmt.Println(fmt.Sprintf("proxy %v to %v, use %v cpus", targets, port, nbCpus));

    // the context for go sharp.
    ctx := NewGoSharpContext(targets)

    // max cpus to use
    runtime.GOMAXPROCS(nbCpus)

    // the static dir dispatch router.
    http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
        //fmt.Println("server", req.RemoteAddr)
        proxy_server := ctx.ChooseBest()
        proxy_server.DoProxy(w, req)
    })

    if err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
        fmt.Println("http serve failed and err is", err)
        return -1
    }

    return 0
}

func main() {
    ret := goSharpRun()
    os.Exit(ret)
}