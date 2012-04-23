package main

/* stoeln from
   https://github.com/ThePiachu/Go-HTTP-JSON-RPC/blob/master/httpjsonrpc/httpjsonrpcClient.go
   because the standard library stinks */

import (
        "encoding/json"
        "io/ioutil"
        "log"
        "net/http"
        "os"
        "strings"
)

func main() {
        host := os.Args[1]
        port := os.Args[2]

        address := "http://" + host + ":" + port + "/jsonrpc"

        data, err := json.Marshal(map[string]interface{}{
                "method": "VideoLibrary.Scan",
                "id":     "xbmc-library-update",
                "jsonrpc": "2.0",
        })

        resp, err := http.Post(address, "application/json", strings.NewReader(string(data)))
        if err != nil {
                log.Fatalf("Post: %v", err)
        }

        defer resp.Body.Close()

        body, err := ioutil.ReadAll(resp.Body)

        if err != nil {
                log.Fatalf("ReadAll: %v", err)
        }

        result := make(map[string]interface{})

        err = json.Unmarshal(body, &result)

        if result["result"] != "OK" {
                os.Exit(1)
        }

        os.Exit(0)
}
