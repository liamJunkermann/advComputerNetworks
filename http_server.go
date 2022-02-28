package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/golang/glog"
)

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
			dst.Add(k, v);
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

type proxy struct {}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	glog.Info(req.RemoteAddr, " ", req.Method, " ", req.URL, "Host: ", req.Host)
	requestDump, _ := httputil.DumpRequest(req, true)
	glog.Info(string (requestDump))
	
	
	if(req.Method != "CONNECT") { // if HTTP request
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
		}
		defer resp.Body.Close()

		glog.Info(req.RemoteAddr, " ", resp.Status)

		delHopHeaders(resp.Header)

		copyHTTPHeader(wr.Header(), resp.Header)
		wr.WriteHeader(resp.StatusCode)
		io.Copy(wr, resp.Body)
	} else {
		glog.Info(strings.Index(req.Host, ":"))
		if (!strings.Contains(req.Host, ":")) {
			req.Host += ":80"
		}
		// Server connection
		serverConn, err := net.Dial("tcp", req.Host)
		glog.Info(err);

		// Access Client connection
		hj, _ := wr.(http.Hijacker)
		clientConn, _, hjErr := hj.Hijack()
		glog.Info(hjErr)

		clientConn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
		go io.Copy(clientConn, serverConn)
		_, srvErr := io.Copy(serverConn, clientConn)
		glog.Info(srvErr)
	}
}

func main() {
	var addr = flag.String("addr", "127.0.0.1:8080", "The addr of the application.")
	flag.Parse()

	handler := &proxy{}

	glog.Info("Starting proxy server on ", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		glog.Fatal("ListenAndServe:", err)
	}
}
