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
    "time"
    "io/ioutil"
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
    Available bool
}

func (v *GoSharpNode) DoProxy(w http.ResponseWriter, req *http.Request) {
    proxy_url := fmt.Sprintf("http://127.0.0.1:%v%v", v.URL, req.RequestURI)
    fmt.Println(fmt.Sprintf("serve %v, proxy http://%v%v to %v", req.RemoteAddr, req.Host, req.RequestURI, proxy_url))

    if proxy_req,err := http.Get(proxy_url); err != nil {
        fmt.Println(fmt.Sprintf("serve %v, proxy failed, err is %v", req.RemoteAddr, err))
        return
    } else {
        written,_ := io.Copy(w, proxy_req.Body)
        fmt.Println(fmt.Sprintf("server %v, proxy completed, written is %v", req.RemoteAddr, written))
    }
}

func (v *GoSharpNode) Detect() (err error) {
    url := fmt.Sprintf("http://127.0.0.1:%v/api/v1/versions", v.URL)
    //fmt.Println("detect node", url)

    var res *http.Response
    if res,err = http.Get(url); err != nil {
        v.Available = false
        return
    }
    defer res.Body.Close()

    var srs []byte
    if srs,err = ioutil.ReadAll(res.Body); err != nil {
        v.Available = false
        return
    }

    if strings.Contains(string(srs), "version") {
        v.Available = true
    }
    //fmt.Println("detect node", url, "status is", string(srs))

    return
}

type GoSharpContext struct {
    nodes []string
    // key: id of node
    lb map[string]*GoSharpNode
}

func NewGoSharpContext(servers []string) *GoSharpContext {
    v := &GoSharpContext{}

    v.nodes = servers
    v.lb = make(map[string]*GoSharpNode)

    for _,server := range servers {
        node := &GoSharpNode{}
        node.ID = server
        node.URL = server
        node.Available = true
        v.lb[node.ID] = node
    }

    return v
}

func (v *GoSharpContext) ChooseBest() *GoSharpNode {
    var match *GoSharpNode

    for _,node := range v.lb {
        if !node.Available {
            continue
        }

        if match == nil || match.Load > node.Load {
            match = node
        }
    }

    return match
}

func (v *GoSharpContext) Detect() (err error) {
    defer func() {
        if r := recover(); r != nil {
            switch r := r.(type) {
                case error:
                err = r
                default:
                fmt.Println("unknown panic", r)
            }
        }
    } ()

    //fmt.Println("auto detect nodes", v.nodes)
    report := "auto detect: "
    for _,node := range v.lb {
        // ignore any error.
        node.Detect()

        status := "online"
        if !node.Available {
            status = "offline"
        }
        report = fmt.Sprintf("%v%v(%v), ", report, node.ID, status)
    }
    fmt.Println(report)

    return
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
        node := ctx.ChooseBest()

        if node == nil {
            fmt.Println("no online node.")
            return
        }

        // update the load.
        node.Load++
        defer func() {
            node.Load--
        } ()

        // do proxy.
        node.DoProxy(w, req)
    })

    // detect the status of all SRS.
    go func() {
       defer func() {
           for {
               if err := ctx.Detect(); err != nil {
                   fmt.Println("context sync server error:", err)
               }
               time.Sleep(time.Duration(3) * time.Second)
           }
       }()
    }()

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