package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"v2ray-speedtest/lib"
)

var dir string
var wg sync.WaitGroup
var goos string

type proxyNode struct {
	Name  string
	Proxy string
}

var proxySlice []proxyNode

func main() {
	var err error
	goos = runtime.GOOS
	dir, err = os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("获取项目路径出错：%v\n", err.Error()))
	}
	r, err := lib.Request("https://sub.wild233.cf/link/20D8L4YfgsoR9WeB?sub=3", "", 15*time.Second)
	if err != nil {
		panic(fmt.Sprintf("获取节点订阅出错：%v\n", err.Error()))
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

func KillProcess() {
	//结束代理进程
	_, _ = exec.Command("cmd", "/c", "taskkill", "/f", "/im", "xray.exe").Output()
	//删除配置文件
	_, _ = exec.Command("cmd", "/c", "del", fmt.Sprintf("%v\\client\\config\\*.json", dir)).Output()
}

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
	cmd := fmt.Sprintf("%v/client/%v/xray.exe -config=%v/client/config/%v.json", dir, goos, dir, index)
	cmdR := exec.Command("cmd", "/c", cmd)
	err = cmdR.Start()
	if err != nil {
		lib.Log().Error("启动代理客户端出错：%v\n%v\n%v", index, err.Error(), cmdR.Args)
		return
	}
	//lib.Log().Info("启动代理客户端成功，PID为：%v，节点为：%v\n", r.Process.Pid, name)
	r, err := lib.Request("https://www.google.com", fmt.Sprintf("socks5://127.0.0.1:%v", port), 5*time.Second)
	if err != nil {
		//lib.Log().Error("节点连接失败：[%v]\n%v", name, err.Error())
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
