package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var addr = flag.String("addr", ":8081", "http service address")
var hub = newHub()

func serveHome(w http.ResponseWriter, r *http.Request) {
	//log.Println(r.URL.Path)
	//log.Println(r.Header.Get("Origin"))

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.URL.Path != "/init" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// http.ServeFile(w, r, "home.html")
	//prevData := GetPrevData()
	// w.WriteHeader(http.StatusCreated)
}

func main() {
	msgCh := make(chan string)
	flag.Parse()
	go hub.run()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		client := serveWs(hub, w, r)

		prevRank, prevUp := GetPrevData()
		tmpdata := map[string]interface{}{
			"rank": prevRank,
			"up":   prevUp,
		}
		b, err := json.Marshal(tmpdata)
		_ = b
		if err != nil {
			fmt.Printf("Error: %s", err)
			return
		}
		(*client).send <- []byte(b)
	})
	go doEvery(10*time.Second, work)
	err := http.ListenAndServeTLS(*addr, "server.crt", "server.key", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
		msgCh <- "String"
	}
}

func doEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
	}
}

func work(t time.Time) {
	rank, up, err := GetLiveData() // AfreecaData{}
	if err != nil {
		log.Println(err.Error())
	}
	tmpdata := map[string]interface{}{
		"rank": rank,
		"up":   up,
	}
	b, err := json.Marshal(tmpdata)
	_ = b
	if err != nil {
		fmt.Printf("Error: %s", err)
		return
	}
	hub.broadcast <- []byte(b)

	//PrintMemUsage()
}
