package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang/glog"
)

type WebDashboard struct{}

func (f *WebDashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	glog.Info(r.Method, " request from ", r.Host, " for ", r.URL.Path)
	

	if r.URL.Path == "/urls" && r.Method == "GET" {
		glog.Info("handling urls")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(urlList.UrlValues)
	} else {
		switch r.Method {
		case "GET":
			http.ServeFile(w, r, "index.html")
		case "POST":
			//TODO: HANDLE POST REQUEST
			bodyBytes, err := io.ReadAll(r.Body)
			bodyString := string(bodyBytes)
			if err != nil {
				fmt.Fprintf(w, "Sorry, an error occurred reading the body: %s", err.Error())
			}
			switch r.URL.Path {
			case "/block":
				glog.Info("body:")
				glog.Info(bodyString)
				urlList.block(bodyString)
				json.NewEncoder(w).Encode(urlList.UrlValues[bodyString])
				break
			case "/unblock":
				glog.Info("body:")
				glog.Info(bodyString)
				urlList.unblock(bodyString)
				json.NewEncoder(w).Encode(urlList.UrlValues[bodyString])
			}
		default:
			fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
		}
	}
}
