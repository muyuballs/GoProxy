package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sort"
	"time"
)

type ServConfig struct {
	Port    int
	Ssl     bool
	SslPort int
	Cert    string
	Key     string
	Host    string
	Buffer  int
}

type StringArray []string

var (
	servConfig = ServConfig{}
	blackList  = StringArray(make([]string, 0))
	transport  = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}
)

func (strArray StringArray) IndexOf(str string) int {
	for i, x := range strArray {
		if x == str {
			return i
		}
	}
	return -1
}

func loadConfig() (err error) {
	data, err := ioutil.ReadFile("conf/serv.json")
	if err == nil {
		err = json.Unmarshal(data, &servConfig)
	}
	data, err = ioutil.ReadFile("conf/black.json")
	if err == nil {
		err = json.Unmarshal(data, &blackList)
		if err == nil {
			sort.Strings(blackList)
		}
	}
	return
}

func transfer(r io.ReadCloser, w io.Writer) (err error) {
	if r != nil && w != nil {
		buf := make([]byte, servConfig.Buffer)
		for {
			c, err := r.Read(buf)
			if err != nil {
				if err == io.EOF {
					w.Write(buf[:c])
				}
				break
			}
			w.Write(buf[:c])
		}
	}
	return
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s  %s  %s\n", r.RemoteAddr, r.Proto, r.Method, r.URL)
	for k, v := range r.Header {
		log.Printf("%s:%s\n", k, v)
	}
	if blackList.IndexOf(r.Host) > -1 {
		log.Printf("%s in blacklist.\n", r.Host)
		w.WriteHeader(http.StatusTeapot)
		return
	}

	resp, err := transport.RoundTrip(r)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	for _, c := range resp.Cookies() {
		w.Header().Add("Set-Cookie", c.Raw)
	}
	w.WriteHeader(resp.StatusCode)
	err = transfer(resp.Body, w)
	if err != nil && err != io.EOF {
		panic(err)
	}
}

func main() {
	err := loadConfig()
	if err != nil {
		panic(err)
		return
	}
	config, _ := json.MarshalIndent(servConfig, "", " ")
	log.Println("Server Config:\n", string(config))
	blist, _ := json.MarshalIndent(blackList, "", " ")
	log.Println("Black List:\n", string(blist))
	host := fmt.Sprintf("%s:%d", servConfig.Host, servConfig.Port)
	sslHost := fmt.Sprintf("%s:%d", servConfig.Host, servConfig.SslPort)
	http.HandleFunc("/", handler)
	if servConfig.Ssl {
		go func() {
			log.Printf("Start ssl serving on %s\n", sslHost)
			http.ListenAndServeTLS(sslHost, servConfig.Cert, servConfig.Key, nil)
		}()
	}
	log.Printf("Start serving on %s\n", host)
	http.ListenAndServe(host, nil)
}
