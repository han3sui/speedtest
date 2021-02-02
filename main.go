package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"time"
	"v2ray-speedtest/lib"
)

var dir string
var wg sync.WaitGroup
var clientPath string

type proxyNode struct {
	Name  string
	Proxy string
}

var proxySlice []proxyNode

func main() {
	go GetSingle()
	var err error
	dir, err = os.Getwd()
	if err != nil {
		lib.Log().Error("获取项目路径出错：%v\n", err.Error())
		os.Exit(0)
	}
	if runtime.GOARCH == "amd64" {
		if runtime.GOOS == "windows" {
			clientPath = fmt.Sprintf("%v/client/xray.exe", dir)
		}
		if runtime.GOOS == "linux" {
			clientPath = fmt.Sprintf("%v/client/xray-linux64", dir)
		}
	}
	if runtime.GOARCH == "arm64" && runtime.GOOS == "linux" {
		clientPath = fmt.Sprintf("%v/client/xray-arm64", dir)
	}
	if clientPath == "" {
		lib.Log().Error("目前仅支持windows 64位，linux 64位系统")
		os.Exit(0)
	}
	err = lib.CreatePath(fmt.Sprintf("%v/client/config", dir))
	if err != nil {
		lib.Log().Error("创建配置文件夹失败")
		os.Exit(0)
	}
	subscribe := CreateFlag()
	r, err := lib.Request(subscribe, "", 15*time.Second)
	if err != nil {
		lib.Log().Error("获取节点订阅出错：\n%v", err.Error())
		os.Exit(0)
	}
	base64DecodeC, _ := base64.StdEncoding.DecodeString(r.Body)
	sliceBase64 := strings.Split(string(base64DecodeC), "\n")
	KillProcess()
	for i, v := range sliceBase64 {
		if strings.Contains(v, "vmess://") {
			s := strings.Replace(v, "vmess://", "", -1)
			sByte, _ := base64.StdEncoding.DecodeString(s)
			sMap := map[string]interface{}{}
			err := json.Unmarshal(sByte, &sMap)
			if err != nil {
				lib.Log().Error("序列化节点出错")
			} else {
				_, _ = CreateConfigFile(i, sMap)
			}
		}
	}
	wg.Wait()
	for _, v := range proxySlice {
		lib.Log().Info("测速节点：[%v]", v.Name)
		_, _, err = lib.Download("http://mirror.hk.leaseweb.net/speedtest/10000mb.bin", v.Proxy)
		if err != nil {
			lib.Log().Error("请求测速文件失败：%v", err.Error())
		}
	}
	KillProcess()
}

func GetSingle() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan)
	select {
	case <-sigChan:
		KillProcess()
		os.Exit(1)
	}
}

func CreateFlag() string {
	var (
		subscribe string
		h         bool
	)
	flag.StringVar(&subscribe, "u", "", "vmess订阅链接地址")
	flag.BoolVar(&h, "h", false, "使用说明")
	flag.Parse()
	if h {
		_, _ = fmt.Fprintf(os.Stderr, `v2ray-speedtest version: 1.0.0

Options:
`)
		flag.PrintDefaults()
		os.Exit(0)
	}
	if subscribe == "" {
		if len(flag.Args()) == 0 {
			lib.Log().Error("请输入订阅地址")
			os.Exit(0)
		} else {
			subscribe = flag.Args()[0]
		}
	}
	return subscribe
}

func KillProcess() {
	//结束代理进程
	_, _ = exec.Command("cmd", "/c", "taskkill", "/f", "/im", "xray.exe").Output()
	//删除配置文件
	_, _ = exec.Command("cmd", "/c", "del", fmt.Sprintf("%v\\client\\config\\*.json", dir)).Output()
}

//{
//"v": "2",                                       // vmess:// 版本
//"ps": "aliasName",                              // 自定义名称
//"add": "111.111.111.111",                       // 服务器域名或 IP
//"port": "32000",                                // 端口号
//"id": "1386f85e-657b-4d6e-9d56-78badb75e1fd",   // VMess UID
//"aid": "100",                                   // VMess AlterID
//"net": "tcp",                                   // 传输设置 tcp\kcp\ws\h2\quic
//"type": "none",                                 // 伪装设置 none\http\srtp\utp\wechat-video
//"host": "www.bbb.com",                          // host (HTTP, WS, H2) 或 security (QUIC)
//"path": "/",                                    // path (WS, H2) 或 key (QUIC)
//"tls": "tls"                                    // tls 设置
//}
//VMess:// 协议格式
//vmess:// + BASE64Encode(以上JSON)
func CreateConfigFile(index int, node map[string]interface{}) (proxy string, err error) {
	name := strings.TrimSpace(fmt.Sprintf("%v", node["ps"]))
	configDir := fmt.Sprintf("%v/client/config/%v.json", dir, index)
	add := fmt.Sprintf("%v", node["add"])
	aid := fmt.Sprintf("%v", node["aid"])
	//host := fmt.Sprintf("%v", node["host"])
	id := fmt.Sprintf("%v", node["id"])
	net := fmt.Sprintf("%v", node["net"])
	path := fmt.Sprintf("%v", node["path"])
	port := fmt.Sprintf("%v", node["port"])
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
      "security": "none",
      "wsSettings": {
        "headers": {
          "Host": "%v"
        },
        "path": "%v"
      }
    }
  }
}
`, 2000+index, add, port, id, aid, net, "", path)
	err = ioutil.WriteFile(configDir, []byte(tmp), 0644)
	if err != nil {
		lib.Log().Error("配置文件创建失败[%v]\n%v", name, err.Error())
		return
	}
	wg.Add(1)
	go func() {
		proxy, err = ExecProxyCore(index, 2000+index, name)
	}()
	return
}

func ExecProxyCore(index int, port int, name string) (proxy string, err error) {
	defer wg.Done()
	cmd := fmt.Sprintf("%v -config=%v/client/config/%v.json", clientPath, dir, index)
	cmdR := exec.Command("cmd", "/c", cmd)
	err = cmdR.Start()
	if err != nil {
		lib.Log().Error("启动代理客户端出错：%v\n%v\n%v", index, err.Error(), cmdR.Args)
		return
	}
	//lib.Log().Info("启动代理客户端成功，PID为：%v，节点为：%v\n", r.Process.Pid, name)
	r, err := lib.Request("https://www.google.com", fmt.Sprintf("socks5://127.0.0.1:%v", port), 5*time.Second)
	if err != nil {
		lib.Log().Error("节点连接失败：[%v]\n%v", name, err.Error())
		return
	}
	ipBody, err := lib.Request("https://myip.ipip.net/", r.Proxy, 5*time.Second)
	if err != nil {
		lib.Log().Error("获取节点：[%v] ip失败:\n%v", name, err.Error())
		return
	}
	lib.Log().Info("节点连接成功：[%v]，请求次数：%v，耗时：%v\n%v", name, 6-r.Retries, r.Duration, ipBody.Body)
	proxySlice = append(proxySlice, proxyNode{
		Name:  name,
		Proxy: proxy,
	})
	return
}
