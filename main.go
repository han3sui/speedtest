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
	"v2ray-speedtest/lib"
)

var dir string
var wg sync.WaitGroup
var goos string

func main() {
	var err error
	goos = runtime.GOOS
	dir, err = os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("获取项目路径出错：%v\n", err.Error()))
	}
	r, err := lib.Request("https://sub.wild233.cf/link/20D8L4YfgsoR9WeB?sub=3", "")
	if err != nil {
		panic(fmt.Sprintf("获取节点订阅出错：%v\n", err.Error()))
	}
	base64DecodeC, _ := base64.StdEncoding.DecodeString(r.Body)
	sliceBase64 := strings.Split(string(base64DecodeC), "\n")
	KillProcess()
	var proxySlice []string
	for i, v := range sliceBase64 {
		if strings.Contains(v, "vmess://") {
			s := strings.Replace(v, "vmess://", "", -1)
			sByte, _ := base64.StdEncoding.DecodeString(s)
			sMap := map[string]interface{}{}
			err := json.Unmarshal(sByte, &sMap)
			if err != nil {
				lib.Log().Error("序列化节点出错")
			} else {
				proxy, err := CreateConfigFile(i, sMap)
				if err != nil {
					proxySlice = append(proxySlice, proxy)
				}
			}
		}
	}
	wg.Wait()
	fmt.Printf("%v", proxySlice)
	KillProcess()
}

func KillProcess() {
	//结束v2ray进程
	_, _ = exec.Command("cmd", "/c", "taskkill", "/f", "/im", "v2ray.exe").Output()
	//删除配置文件
	_, _ = exec.Command("cmd", "/c", "del", fmt.Sprintf("%v\\client\\config\\*.json", dir)).Output()
	_, _ = exec.Command("cmd", "/c", "del", fmt.Sprintf("%v\\client\\config\\*.pb", dir)).Output()
}

func CreateConfigFile(index int, node map[string]interface{}) (proxy string, err error) {
	name := strings.TrimSpace(fmt.Sprintf("%v", node["ps"]))
	configDir := fmt.Sprintf("%v/client/config/%v.json", dir, index)
	add := fmt.Sprintf("%v", node["add"])
	aid := fmt.Sprintf("%v", node["aid"])
	host := fmt.Sprintf("%v", node["host"])
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
`, 2000+index, add, port, id, aid, net, host, path)
	err = ioutil.WriteFile(configDir, []byte(tmp), 0644)
	if err != nil {
		lib.Log().Error("配置文件创建失败[%v]\n%v", name, err.Error())
		return
	}
	err = exec.Command("cmd", "/c", fmt.Sprintf("%v\\client\\%v\\v2ctl.exe config %v\\client\\config\\%v.json > %v\\client\\config\\%v.pb", dir, goos, dir, index, dir, index)).Run()
	if err != nil {
		lib.Log().Error("启动v2ray失败[%v]\n%v", name, err.Error())
		return
	}
	wg.Add(1)
	go func() {
		proxy, err = ExecV2rayCore(index, 2000+index, name)
		//fmt.Printf(proxy)
	}()
	return
}

func ExecV2rayCore(index int, port int, name string) (proxy string, err error) {
	defer wg.Done()
	cmd := fmt.Sprintf("%v/client/%v/v2ray.exe -config=%v/client/config/%v.pb -format=pb", dir, goos, dir, index)
	cmdR := exec.Command("cmd", "/c", cmd)
	err = cmdR.Start()
	if err != nil {
		lib.Log().Error("启动v2ray出错：%v\n%v\n%v", index, err.Error(), cmdR.Args)
		return
	}
	//lib.Log().Info("启动v2ray成功，PID为：%v，节点为：%v\n", r.Process.Pid, name)
	r, err := lib.Request("https://www.google.com.hk/", fmt.Sprintf("socks5://127.0.0.1:%v", port))
	if err != nil {
		lib.Log().Error("节点连接失败：[%v]\n%v", name, err.Error())
		return
	}
	ipBody, err := lib.Request("https://myip.ipip.net/", r.Proxy)
	if err != nil {
		lib.Log().Error("获取节点：[%v] ip失败:\n%v", name, err.Error())
		return
	}
	lib.Log().Info("节点连接成功：[%v]，请求次数：%v，耗时：%v\n%v", name, 6-r.Retries, r.Duration, ipBody.Body)
	proxy = r.Proxy
	return
}
