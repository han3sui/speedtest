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
	"sort"
	"strconv"
	"time"
)

type Response struct {
	Body     string
	Duration time.Duration
	Retries  int
	Proxy    string
}

type RequestInfo struct {
	FirstByteDuration time.Duration
	DnsDuration       time.Duration
	ConnectDuration   time.Duration
}

type Reader struct {
	io.Reader
	Total      int
	Current    int
	LastWtn    int
	Written    int
	Max        int
	Avg        int
	SpeedSlice []int
	Ticker     *time.Ticker
}

func Request(url string, proxy string, timeout time.Duration) (response *Response, err error) {
	response = new(Response)
	client, err := CreateClient(proxy, timeout)
	if err != nil {
		return
	}
	request, _, err := CreateRequest(url)
	if err != nil {
		return
	}
	start := time.Now()
	response.Retries = 5
	var rError error
	for response.Retries > 0 {
		r, err := client.Do(request)
		if err != nil {
			rError = err
			response.Retries = response.Retries - 1
		} else {
			defer func() {
				_ = r.Body.Close()
			}()
			response.Duration = time.Since(start)
			bodyByte, _ := ioutil.ReadAll(r.Body)
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

func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	//已写入数据总量
	r.Written += n
	//fmt.Printf("时间：%v，本次数据量：%v，已写入数据量：%v，错误：%v\n", time.Now().Format("2006-01-02 15:04:05"), n, written, err)
loop:
	for {
		if err != nil {
			break loop
		}
		if r.LastWtn-r.Total == 0 {
			break loop
		}
		select {
		case <-r.Ticker.C:
			speed := r.Written - r.LastWtn
			r.SpeedSlice = append(r.SpeedSlice, speed)
			sort.SliceStable(r.SpeedSlice, func(i, j int) bool {
				return r.SpeedSlice[i] > r.SpeedSlice[j]
			})
			r.Avg = r.Written / len(r.SpeedSlice)
			r.Max = r.SpeedSlice[0]
			fmt.Printf("\r时间：%v，完成：%.2f%%，平均下载速度：%v/s，最大下载速度：%v/s", time.Now().Format("2006-01-02 15:04:05"), float64(r.Written*10000/r.Total)/100, BytesToSize(r.Avg), BytesToSize(r.Max))
			r.LastWtn = r.Written
			break loop
		default:
			break loop
		}
	}
	return
}

func Download(url string, proxy string, duration int) (avg int, max int, err error) {
	client, err := CreateClient(proxy, time.Duration(duration)*time.Second)
	if err != nil {
		return
	}
	request, _, err := CreateRequest(url)
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
			reader := &Reader{
				Reader:     r.Body,
				Total:      int(r.ContentLength),
				Written:    0,
				LastWtn:    0,
				Ticker:     time.NewTicker(1 * time.Second),
				SpeedSlice: []int{},
			}
			defer func() {
				max = reader.Max
				avg = reader.Avg
				_ = r.Body.Close()
				reader.Ticker.Stop()
			}()
			_, _ = io.Copy(ioutil.Discard, reader)
			fmt.Printf("\n\n")
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

func CreateRequest(url string) (request *http.Request, info RequestInfo, err error) {
	request, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	request.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36")
	var connect, start, dns time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(dnsStartInfo httptrace.DNSStartInfo) {
			dns = time.Now()
		},
		DNSDone: func(dnsDoneInfo httptrace.DNSDoneInfo) {
			info.DnsDuration = time.Since(dns)
		},
		ConnectStart: func(network, addr string) {
			connect = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			info.ConnectDuration = time.Since(connect)
		},
		GotFirstResponseByte: func() {
			info.FirstByteDuration = time.Since(start)
		},
	}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))
	return
}
