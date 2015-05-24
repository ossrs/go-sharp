package main

import (
    "fmt"
    "os"
    "strconv"
    "errors"
    "runtime"
    "net/http"
    "strings"
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

func goSharpRun() int {
    port, nbCpus, pports, err := goSharpParseOptions()
    if err != nil {
        fmt.Println(err)
        return -1
    }

    // the target SRS to proxy to
    targets := strings.Split(pports, ",")
    fmt.Println(fmt.Sprintf("proxy %v to %v, use %v cpus", targets, port, nbCpus));

    // max cpus to use
    runtime.GOMAXPROCS(nbCpus)

    // the static dir dispatch router.
    http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
        return
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