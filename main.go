package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	_ = request("https://www.google.com.hk/", "socks5://127.0.0.1:2000")
	file, err := os.Open("node.txt")
	if err != nil {
		panic(err)
	}
	c, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	base64DecodeC, _ := base64.StdEncoding.DecodeString(string(c))
	strBase64 := string(base64DecodeC)
	sliceBase64 := strings.Split(strBase64, "\n")
	for _, v := range sliceBase64 {
		if strings.Contains(v, "vmess://") {
			s := strings.Replace(v, "vmess://", "", -1)
			sByte, _ := base64.StdEncoding.DecodeString(s)
			sMap := map[string]interface{}{}
			err = json.Unmarshal(sByte, &sMap)
			if err != nil {
				fmt.Printf("序列化节点出错")
			}
			createConfigFile(sMap)
			//fmt.Printf("%v\n", sMap)
		}
	}
}

func createConfigFile(node map[string]interface{}) {
	name := strings.TrimSpace(fmt.Sprintf("%v.json", node["ps"]))
	add := fmt.Sprintf("%v", node["add"])
	aid := fmt.Sprintf("%v", node["aid"])
	host := fmt.Sprintf("%v", node["host"])
	id := fmt.Sprintf("%v", node["id"])
	net := fmt.Sprintf("%v", node["net"])
	path := fmt.Sprintf("%v", node["path"])
	port := fmt.Sprintf("%v", node["port"])
	file, err := os.Stat(name)
	if err != nil || file.IsDir() {
		tmp := fmt.Sprintf(`
{
  "inbound": {
    "port": 2000,
    "listen": "127.0.0.1",
    "protocol": "socks",
    "sniffing": {
      "enabled": true,
      "destOverride": [
        "http",
        "tls"
      ]
    },
    "settings": {
      "auth": "noauth",
      "udp": true
    }
  },
  "outbound": {
    "protocol": "vmess",
    "settings": {
      "vnext": [
        {
          "address": "%v",
          "port": %v,
          "users": [
            {
              "id": "%v",
              "alterId": %v
            }
          ]
        }
      ]
    },
    "mux": {
      "enabled": false
    },
    "streamSettings": {
      "network": "%v",
      "wsSettings": {
        "headers": {
          "Host": "%v"
        },
        "path": "%v"
      },
      "security": "none"
    }
  }
}
`, add, port, id, aid, net, host, path)
		err := ioutil.WriteFile(name, []byte(tmp), 0644)
		if err != nil {
			fmt.Printf("配置文件创建失败[%v]\n%v\n", name,err.Error())
		}
	}
}

func execV2rayCore() {

}

//func main() {
//	r := client("https://sub.wild233.cf/link/20D8L4YfgsoR9WeB?sub=3")
//	body, _ := ioutil.ReadAll(r.Body)
//	base64Decode, _ := base64.StdEncoding.DecodeString(string(body))
//	base64DecodeStr := string(base64Decode)
//	base64DecodeSlice := strings.Split(base64DecodeStr, "\r\n")
//	fmt.Println(base64DecodeSlice)
//}
//
func request(urlLink string, proxyStr string) (r *http.Response) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("PANIC RECOVER:\n%v", err)
		}
	}()
	proxyUrl, err := url.Parse(proxyStr)
	if err != nil {
		panic(err)
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Proxy: http.ProxyURL(proxyUrl),
		},
	}
	request, err := http.NewRequest("GET", urlLink, nil)
	if err != nil {
		panic(err)
	}
	request.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36")
	var start, connect, dns time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(dnsStartInfo httptrace.DNSStartInfo) {
			dns = time.Now()
		},
		DNSDone: func(dnsDoneInfo httptrace.DNSDoneInfo) {
			fmt.Printf("DNS DONE: %v\n", time.Since(dns))
		},
		ConnectStart: func(network, addr string) {
			connect = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			fmt.Printf("Connect Time: %v\n", time.Since(connect))
		},
		GotFirstResponseByte: func() {
			fmt.Printf("Time from start to first byte: %v\n", time.Since(start))
		},
	}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))
	start = time.Now()
	r, err = client.Do(request)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Total time: %v\n", time.Since(start))
	return r
}
