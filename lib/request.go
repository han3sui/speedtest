package lib

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Response struct {
	Body     string
	Duration time.Duration
	Retries  int
	Client   *http.Client
	Proxy    string
}

type Reader struct {
	io.Reader
	Total   int
	Current int
}

func Request(url string, proxy string, timeout time.Duration) (response *Response, err error) {
	response = new(Response)
	response.Client, err = CreateClient(proxy, timeout)
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

var lastWtn int
var written int
var ticker *time.Ticker

func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	//已写入数据总量
	written += n
	//fmt.Printf("时间：%v，本次数据量：%v，已写入数据量：%v，错误：%v\n", time.Now().Format("2006-01-02 15:04:05"), n, written, err)
loop:
	for {
		if err != nil {
			break loop
		}
		select {
		case <-ticker.C:
			speed := written - lastWtn
			fmt.Printf("\r时间：%v，完成：%.2f%%", time.Now().Format("2006-01-02 15:04:05"), float64(written*10000/r.Total)/100)
			fmt.Printf("，速度：%v/s\n", BytesToSize(speed))
			lastWtn = written
			break loop
		default:
			break loop
		}
	}
	return
}

func Download(url string, proxy string, filename string) (err error) {
	written = 0
	lastWtn = 0
	ticker = time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	client, err := CreateClient(proxy, 15*time.Second)
	if err != nil {
		return
	}
	request, err := CreateRequest(url)
	if err != nil {
		return
	}
	retries := 5
	var rError error
	for retries > 0 {
		r, err := client.Do(request)
		if err != nil {
			rError = err
			retries = retries - 1
		} else {
			defer r.Body.Close()
			out, _ := os.Create(filename)
			defer out.Close()
			reader := &Reader{
				Reader: r.Body,
				Total:  int(r.ContentLength),
			}
			_, _ = io.Copy(out, reader)
			break
		}
	}
	if retries == 0 {
		err = rError
		return
	}
	return
}
func BytesToSize(length int) string {
	var k = 1024
	var sizes = []string{"Bytes", "KB", "MB", "GB", "TB"}
	if length == 0 {
		return "0 Bytes"
	}
	i := math.Floor(math.Log(float64(length)) / math.Log(float64(k)))
	r := float64(length) / math.Pow(float64(k), i)
	return strconv.FormatFloat(r, 'f', 3, 64) + " " + sizes[int(i)]
}

func CreateClient(proxy string, timeout time.Duration) (client *http.Client, err error) {
	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		Log().Error("创建客户端错误，%s", err.Error())
		return
	}
	client = &http.Client{
		Timeout: timeout,
	}
	if proxy != "" {
		client = &http.Client{
			Timeout: timeout,
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
