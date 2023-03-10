package main

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// renderJSON renders 'v' as JSON and writes it as a response into w.
func renderJSON(w http.ResponseWriter, v interface{}) {
	js, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func main() {
	hello := func(response http.ResponseWriter, request *http.Request) {
		if _, err := response.Write([]byte("hello world\n")); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	echo := func(response http.ResponseWriter, request *http.Request) {
		rawReq, err := httputil.DumpRequest(request, true)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := response.Write(rawReq); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	cpu := func(response http.ResponseWriter, request *http.Request) {
		cpuinfos, err := cpu.Info()
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(response, cpuinfos)
	}
	vmem := func(response http.ResponseWriter, request *http.Request) {
		vmem, _ := mem.VirtualMemory()
		if _, err := response.Write([]byte(vmem.String())); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	net := func(response http.ResponseWriter, request *http.Request) {
		ifList, err := net.Interfaces()
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJSON(response, ifList)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", echo)
	mux.HandleFunc("/hello", hello)
	mux.HandleFunc("/cpu", cpu)
	mux.HandleFunc("/mem", vmem)
	mux.HandleFunc("/net", net)

	var addr string = ":8080"
	if envaddr := os.Getenv("LISTEN_ADDR"); envaddr != "" {
		addr = envaddr
	}
	server := &http.Server{Addr: addr, Handler: mux}
	server.ListenAndServe()
}
