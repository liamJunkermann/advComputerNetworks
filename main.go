package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/golang/glog"
)

var config *Config
var cache *Cache

func main() {
	var addr = flag.String("addr", "127.0.0.1:8080", "The addr of the application.")
	configPath := flag.String("config", "./tiny.json", "configuration .json file path")
	flag.Parse()

	loadConfig(*configPath)
	glog.Info("Config Loaded")

	prepare()

	handler := &proxy{}

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

func prepare() {
	var err error
	cache, err = CreateCache(config.CacheFolder)

	if err != nil {
		glog.Fatal("Could not init cache: '%s'", err.Error())
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

func copyHTTPHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

type proxy struct{}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	glog.Info(req.RemoteAddr, " ", req.Method, " ", req.URL, "Host: ", req.Host)
	requestDump, _ := httputil.DumpRequest(req, true)
	glog.Info(string(requestDump))

	if req.Method != "CONNECT" { // if HTTP request
		fullUrl := req.Host + req.URL.Path + "?" + req.URL.RawQuery
		glog.Info("Requested: ", fullUrl)
		client := &http.Client{}

		if busy, ok := cache.has(fullUrl); !ok {
			startTime := time.Now()
			defer busy.Unlock()
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

			var reader io.Reader
			reader = resp.Body
			endTime := time.Now()
			totalTime := endTime.Sub(startTime)
			glog.Info("Time Spent: ", totalTime)
			err = cache.put(fullUrl, &reader, resp.ContentLength, totalTime)
			if err != nil {
				http.Error(wr, "Server Error", http.StatusInternalServerError)
				glog.Fatal("ServeHTTP:", err)
				return
			}
			defer resp.Body.Close()
		}

		content, err := cache.get(fullUrl)
		if err != nil {
			http.Error(wr, "Server Error", http.StatusInternalServerError)
			glog.Fatal("Serve from Cache", err)
		} else {
			contentWritten, err := io.Copy(wr, *content)
			if err != nil {
				glog.Fatal("Error writing response: ", err.Error())
				return
			}
			glog.Info("Wrote ", contentWritten, " bytes to client")
		}
	} else {
		glog.Info(strings.Index(req.Host, ":"))
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
		glog.Info(srvErr)
	}
}
