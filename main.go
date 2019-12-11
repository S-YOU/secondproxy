package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/magisterquis/connectproxy"
	"golang.org/x/net/proxy"
)

var (
	proxyStr = flag.String("proxy", "", "proxy url")
	listen   = flag.String("listen", ":3128", "listen address")
	username = flag.String("username", "", "proxy username")
	password = flag.String("password", "", "proxy password")
)

var (
	dialer proxy.Dialer
)

func serveHTTPS(w http.ResponseWriter, r *http.Request) {
	destConn, err := dialer.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}

	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func main() {
	var err error
	flag.Parse()

	proxyURL, err := url.Parse(*proxyStr)
	if err != nil {
		log.Fatal(err)
	}

	dialer, err = connectproxy.New(proxyURL, proxy.Direct)
	if nil != err {
		log.Fatal(err)
	}

	director := func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", req.Host)
		req.URL.Scheme = "http"
		req.URL.Host = proxyURL.Host
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	proxy := &httputil.ReverseProxy{Transport: transport, Director: director}

	server := &http.Server{
		Addr: *listen,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := basicAuth(r)
			if !ok || user != *username || pass != *password {
				http.Error(w, "invalid auth", http.StatusForbidden)
				return
			}

			if r.Method == http.MethodConnect {
				serveHTTPS(w, r)
			} else {
				proxy.ServeHTTP(w, r)
			}
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	log.Fatal(server.ListenAndServe())
}
