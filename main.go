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
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var dir string

var wg sync.WaitGroup

func main() {
	dir, _ = os.Getwd()
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
	//结束xray进程
	_, _ = exec.Command("cmd", "/c", "taskkill", "/f", "/im", "xray.exe").Output()
	//删除配置文件
	_, _ = exec.Command("cmd", "/c", "del", fmt.Sprintf("%v\\client\\config\\*.json", dir)).Output()
	for i, v := range sliceBase64 {
		if strings.Contains(v, "vmess://") {
			s := strings.Replace(v, "vmess://", "", -1)
			sByte, _ := base64.StdEncoding.DecodeString(s)
			sMap := map[string]interface{}{}
			err = json.Unmarshal(sByte, &sMap)
			if err != nil {
				fmt.Printf("序列化节点出错")
			}
			createConfigFile(i, sMap)
			//fmt.Printf("%v\n", sMap)
		}
	}
	wg.Wait()
}

func createConfigFile(index int, node map[string]interface{}) {
	name := strings.TrimSpace(fmt.Sprintf("%v", node["ps"]))
	configDir := fmt.Sprintf("%v/client/config/%v.json", dir, index)
	add := fmt.Sprintf("%v", node["add"])
	aid := fmt.Sprintf("%v", node["aid"])
	host := fmt.Sprintf("%v", node["host"])
	id := fmt.Sprintf("%v", node["id"])
	net := fmt.Sprintf("%v", node["net"])
	path := fmt.Sprintf("%v", node["path"])
	port := fmt.Sprintf("%v", node["port"])
	file, err := os.Stat(configDir)
	if err != nil || file.IsDir() {
		tmp := fmt.Sprintf(`
{
  "inbound": {
    "port": %v,
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
`, 2000+index, add, port, id, aid, net, host, path)
		err := ioutil.WriteFile(configDir, []byte(tmp), 0644)
		if err != nil {
			fmt.Printf("配置文件创建失败[%v]\n%v\n", name, err.Error())
		} else {
			wg.Add(1)
			go execXrayCore(index, 2000+index, name)
		}
	}
}

func execXrayCore(index int, port int, name string) {
	cmd := fmt.Sprintf("%v/client/%v/xray.exe -config %v/client/config/%v.json", dir, runtime.GOOS, dir, index)
	r := exec.Command("cmd", "/c", cmd)
	err := r.Start()
	if err != nil {
		fmt.Printf("启动xray出错：%v\n%v\n%v\n", index, err.Error(), r.Args)
	} else {
		//fmt.Printf("启动xray成功，PID为：%v，节点为：%v\n", r.Process.Pid, name)
		_, _ = request("https://www.google.com.hk/", fmt.Sprintf("socks5://127.0.0.1:%v", port), name, r.Process.Pid)
		//if err != nil {
		//	fmt.Printf("HTTP请求出错：%v\n", err)
		//}
	}
}

func request(urlLink string, proxyStr string, name string, pid int) (body string, err error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("PANIC RECOVER:\n%v\n", err)
			wg.Done()
		}
	}()
	proxyUrl, _ := url.Parse(proxyStr)
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			Proxy: http.ProxyURL(proxyUrl),
		},
	}
	request, err := http.NewRequest("GET", urlLink, nil)
	if err != nil {
		return
	}
	request.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36")
	var start, connect, dns time.Time
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
	start = time.Now()
	r, err := client.Do(request)
	if err != nil {
		fmt.Printf("连接失败，节点：%v\n\n", name)
		err = exec.Command("cmd", "/c", fmt.Sprintf("taskkill /pid %v -f", pid)).Run()
		wg.Done()
		return
	}
	fmt.Printf("连接成功，耗时: %v，节点：%v，配置：%v\n\n", time.Since(start), name, proxyStr)
	bodyByte, _ := ioutil.ReadAll(r.Body)
	body = string(bodyByte)
	wg.Done()
	return
}
