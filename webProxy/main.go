package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/golang/glog"
)

var config *Config

func main() {
	configPath := flag.String("config", "./tiny.json", "configuration .json file path")
	flag.Parse()

	loadConfig(*configPath)
	glog.Info("Config Loaded")

	handler := &proxy{}

	var addr = flag.String("addr", ":"+config.Port, "The addr of the proxy.")

	glog.Info("Starting proxy server on ", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		glog.Fatal("ListenAndServe:", err)
	}
}

func loadConfig(configPath string) {
	var err error

	config, err = LoadConfig(configPath)
	if err != nil {
		glog.Fatal("Could not read config: '%s'", err.Error())
	}
}

var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// Including previous proxy hops in X-Forwarded-For Header
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func copyHTTPHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

type proxy struct{}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	glog.Info(req.RemoteAddr, " ", req.Method, " ", req.URL, " Host: ", req.Host)

	if req.Method != "CONNECT" { // if HTTP request
		if (req.URL.Host == "") {
			http.Error(wr, "Not Found", http.StatusBadRequest)
		} else {
			client := &http.Client{}
	
			req.RequestURI = ""
	
			delHopHeaders(req.Header)
			if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
				appendHostToXForwardHeader(req.Header, clientIP)
			}
	
			resp, err := client.Do(req)
			if err != nil {
				http.Error(wr, "Server Error", http.StatusInternalServerError)
				log.Fatal("ServeHTTP:", err)
				return
			}
			defer resp.Body.Close()
	
			copyHTTPHeader(wr.Header(), resp.Header)
			wr.WriteHeader(resp.StatusCode)
			io.Copy(wr, resp.Body)
		}
	} else {
		// glog.Info(strings.Index(req.Host, ":"))
		if !strings.Contains(req.Host, ":") {
			req.Host += ":80"
		}
		// Server connection
		serverConn, err := net.Dial("tcp", req.Host)
		if err != nil {
			glog.Error(err)
		}

		// Access Client connection
		hj, _ := wr.(http.Hijacker)
		clientConn, _, hjErr := hj.Hijack()
		if hjErr != nil {
			glog.Error(hjErr)
		}

		clientConn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
		go io.Copy(clientConn, serverConn)
		_, srvErr := io.Copy(serverConn, clientConn)
		if srvErr != nil {
			glog.Info(srvErr)
		}
	}
}
