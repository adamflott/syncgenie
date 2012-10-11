package main

/* stoeln from
   https://github.com/ThePiachu/Go-HTTP-JSON-RPC/blob/master/httpjsonrpc/httpjsonrpcClient.go
   because the standard library stinks */

import (
	"encoding/json"
	"github.com/voxelbrain/goptions"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func main() {
	options := struct {
		Host          string `goptions:"-h, --host, description='Host to connect to'"`
		Port          int    `goptions:"-p, --port, description='Host port'"`
		User          string `goptions:"-u, --user, description='HTTP simple auth username'"`
		Password      string `goptions:"-w, --password, description='HTTP simple auth password'"`
		goptions.Help `goptions:"-h, --help, description='Show this help'"`

		goptions.Verbs
		Video struct {
			Scan  bool `goptions:"--scan, description='Scan video library'"`
			Clean bool `goptions:"--clean, description='Clean video library'"`
		} `goptions:"video"`
		Audio struct {
			Scan  bool `goptions:"--scan, description='Scan audio library'"`
			Clean bool `goptions:"--clean, description='Clean audio library'"`
		} `goptions:"audio"`
	}{
		Host:     "127.0.0.1",
		Port:     8080,
		User:     "xbmc",
		Password: "xbmc",
	}

	goptions.ParseAndFail(&options)

	address := "http://" + options.Host + ":" + strconv.Itoa(options.Port) + "/jsonrpc"

	method := "VideoLibrary.Scan"

	if options.Video.Clean == true {
		method = "VideoLibrary.Clean"
	} else if options.Audio.Scan == true {
		method = "AudioLibrary.Scan"
	} else if options.Audio.Clean == true {
		method = "AudioLibrary.Clean"
	}

	data, err := json.Marshal(map[string]interface{}{
		"method":  method,
		"id":      "xbmc-library-update",
		"jsonrpc": "2.0",
	})

	client := &http.Client{}

	req, _ := http.NewRequest("POST", address, strings.NewReader(string(data)))

	req.SetBasicAuth(options.User, options.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, resp_err := client.Do(req)

	defer resp.Body.Close()

	if resp_err != nil {
		log.Fatalf("Post: %v", resp_err)
	}

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Fatalf("ReadAll: %v", err)
	}

	result := make(map[string]interface{})

	err = json.Unmarshal(body, &result)

	if result["result"] != "OK" {
		log.Fatalf("Result: %s\n, Error: %v", result["result"], err)
	}

	os.Exit(0)
}
