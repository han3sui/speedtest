package lib

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"
)

type Response struct {
	Body     string
	Duration time.Duration
	Retries  int
	Client   *http.Client
	Proxy    string
}

func Request(url string, proxy string) (response *Response, err error) {
	response = new(Response)
	response.Client, err = CreateClient(proxy)
	if err != nil {
		return
	}
	request, err := CreateRequest(url)
	if err != nil {
		return
	}
	start := time.Now()
	response.Retries = 5
	var rError error
	for response.Retries > 0 {
		r, err := response.Client.Do(request)
		if err != nil {
			rError = err
			response.Retries = response.Retries - 1
		} else {
			response.Duration = time.Since(start)
			bodyByte, _ := ioutil.ReadAll(r.Body)
			_ = r.Body.Close()
			response.Body = string(bodyByte)
			response.Proxy = proxy
			break
		}
	}
	if response.Retries == 0 {
		err = rError
		return
	}
	return
}

func CreateClient(proxy string) (client *http.Client, err error) {
	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		Log().Error("创建客户端错误，%s", err.Error())
		return
	}
	client = &http.Client{
		Timeout: 5 * time.Second,
	}
	if proxy != "" {
		client = &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: http.ProxyURL(proxyUrl),
			},
		}
	}
	return
}

func CreateRequest(url string) (request *http.Request, err error) {
	request, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	request.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36")
	var connect, dns time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(dnsStartInfo httptrace.DNSStartInfo) {
			dns = time.Now()
		},
		DNSDone: func(dnsDoneInfo httptrace.DNSDoneInfo) {
			//fmt.Printf("DNS DONE: %v\n", time.Since(dns))
		},
		ConnectStart: func(network, addr string) {
			connect = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			//fmt.Printf("Connect Time: %v\n", time.Since(connect))
		},
		GotFirstResponseByte: func() {
			//fmt.Printf("Time from start to first byte: %v\n", time.Since(start))
		},
	}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))
	return
}
