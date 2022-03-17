package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

var config *Config
var cache *Cache
var urlList *URLlist

func main() {
	configPath := flag.String("config", "./tiny.json", "configuration .json file path")
	flag.Parse()

	loadConfig(*configPath)
	glog.Info("Config Loaded")

	prepare()

	handler := &proxy{}
	webHandler := &WebDashboard{}

	var addr = flag.String("addr", ":"+config.Port, "The addr of the proxy.")
	var webAddr = flag.String("webAddr", ":"+config.WebPort, "The web dashboard addr.")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		glog.Info("Starting proxy server on ", *addr)
		if err := http.ListenAndServe(*addr, handler); err != nil {
			glog.Fatal("ListenAndServe:", err)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		glog.Info("Starting web server on ", *webAddr)
		if err := http.ListenAndServe(*webAddr, webHandler); err != nil {
			glog.Fatal("ListenAndServe:", err)
		}
		wg.Done()
	}()
	wg.Wait()
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
	urlList, _ = CreateList()

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

type proxy struct{}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	glog.Info(req.RemoteAddr, " ", req.Method, " ", req.URL, " Host: ", req.Host)
	fullUrl := req.Host + req.URL.Path + "?" + req.URL.RawQuery

	_, isListed := urlList.has(fullUrl)
	if !isListed {
		urlList.put(fullUrl, &DynamicBlock{Remoteaddr: req.RemoteAddr, Method: req.Method, Url: req.Host + "" + req.URL.Path, Blocked: false})
	}

	listing, err := urlList.get(fullUrl)

	if err != nil {
		glog.Fatal("Error getting listing")
	}

	glog.Info("Blocked Status: ", listing.Blocked)

	if !listing.Blocked {
		requestDump, _ := httputil.DumpRequest(req, true)
		glog.Info(string(requestDump))

		if req.Method != "CONNECT" { // if HTTP request
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
	} else {
		http.Error(wr, "Proxy Blocked", http.StatusForbidden)
	}
}
