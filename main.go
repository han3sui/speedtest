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
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"v2ray-speedtest/lib"
)

//节点详细信息
type ProxyNode struct {
	Name         string                 //节点名称
	Proxy        string                 //启动代理的地址
	AvgSpeed     int                    //平均下载速度
	MaxSpeed     int                    //最大下载速度
	Ping         time.Duration          //ping谷歌时长
	Status       bool                   //代理状态
	RealIp       string                 //真实IP
	ErrorMessage string                 //出错原因
	Retries      int                    //连接重试次数
	Detail       map[string]interface{} //序列化map详情
}

//命令行交互参数
type Flag struct {
	Subscribe string //订阅地址
	Filter    string //筛选参数
	Help      bool   //调出help
	Duration  int    //下载测速文件时间
}

//程序运行路径
var dir string

//客户端地址
var clientPath string

//节点切片
var ProxySlice []ProxyNode

//命令模式，windows和linux
var commandUse string

//命令参数，windows和linux
var commandArg string

func main() {
	go func() {
		GetSingle()
	}()
	KillProcess()
	var err error
	dir, err = os.Getwd()
	if err != nil {
		lib.Log().Error("获取项目路径出错：%v\n", err.Error())
		os.Exit(0)
	}
	if runtime.GOOS == "windows" {
		commandUse = "cmd"
		commandArg = "/c"
		clientPath = fmt.Sprintf("%v/client/xray.exe", dir)
	} else {
		commandUse = "sh"
		commandArg = "-c"
		clientPath = fmt.Sprintf("%v/client/xray", dir)
	}
	err = lib.CreatePath(fmt.Sprintf("%v/client/config", dir))
	if err != nil {
		lib.Log().Error("创建配置文件夹失败")
		os.Exit(0)
	}
	if !lib.PathExists(clientPath) {
		lib.Log().Error("xray客户端不存在，请先下载系统对应版本客户端，放入client文件夹内，并赋予执行权限")
		os.Exit(0)
	}
	flags := CreateFlag()
	lib.Log().Info("正在请求订阅，请等待...")
	r, err := lib.Request(flags.Subscribe, "", 0)
	if err != nil {
		lib.Log().Error("获取订阅出错：\n%v", err.Error())
		os.Exit(0)
	}
	lib.Log().Info("开始解析订阅内容...")
	base64DecodeC, err := base64.StdEncoding.DecodeString(r.Body)
	if err != nil {
		lib.Log().Error("解析订阅内容出错：\n%v", err.Error())
		os.Exit(0)
	}
	sliceBase64 := strings.Split(string(base64DecodeC), "\n")
	for _, v := range sliceBase64 {
		if strings.Contains(v, "vmess://") {
			s := strings.Replace(v, "vmess://", "", -1)
			sByte, _ := base64.StdEncoding.DecodeString(s)
			sMap := map[string]interface{}{}
			err := json.Unmarshal(sByte, &sMap)
			if err != nil {
				lib.Log().Error("序列化节点出错：%v", v)
			} else {
				name := strings.TrimSpace(fmt.Sprintf("%v", sMap["ps"]))
				boolean := true
				if flags.Filter != "" {
					b1 := false
				loop1:
					for _, v1 := range strings.Split(flags.Filter, "|") {
						//或条件，满足任何一个关键字，加入节点
						if !strings.Contains(v1, "&") {
							//关键字只存在或条件，如果满足任何一个关键字，直接为真，跳出循环
							if strings.Contains(name, v1) {
								b1 = true
								break loop1
							}
						} else {
							//关键字存在与条件，必须满足与条件，才可以为真，跳出循环
							b2 := true
						loop2:
							for _, v2 := range strings.Split(v1, "&") {
								if !strings.Contains(name, v2) {
									b2 = false
									break loop2
								}
							}
							b1 = b2
							if b2 {
								break loop1
							}
						}
					}
					boolean = b1
				}
				if boolean {
					ProxySlice = append(ProxySlice, ProxyNode{
						Detail: sMap,
						Status: true,
					})
				}
			}
		}
	}
	lib.Log().Info("开始创建配置文件...")
	var fileWg sync.WaitGroup
	for i, v := range ProxySlice {
		i1 := i
		v1 := v
		fileWg.Add(1)
		go func() {
			_ = CreateConfigFile(i1, &v1, &fileWg)
		}()
	}
	fileWg.Wait()
	time.Sleep(3 * time.Second)
	lib.Log().Info("开始谷歌连接检查...")
	var googleWg sync.WaitGroup
	for i, v := range ProxySlice {
		if v.Status {
			i1 := i
			v1 := v
			googleWg.Add(1)
			go func() {
				CurlGoogle(i1, &v1, &googleWg)
			}()
		}
	}
	googleWg.Wait()
	lib.Log().Info("开始下载速度测试...")
	for i, v := range ProxySlice {
		if v.Status {
			lib.Log().Info("测速节点：[%v]", v.Name)
			//http://repos.mia.lax-noc.com/speedtests/
			//http://cachefly.cachefly.net/100mb.test
			//http://mirror.hk.leaseweb.net/speedtest/10000mb.bin
			ProxySlice[i].AvgSpeed, ProxySlice[i].MaxSpeed, err = lib.Download("http://mirror.hk.leaseweb.net/speedtest/10000mb.bin", v.Proxy, flags.Duration)
			if err != nil {
				lib.Log().Error("请求测速文件失败：%v", err.Error())
			}
		}
	}
	lib.Log().Info("---------------可用节点测速结果---------------")
	sort.SliceStable(ProxySlice, func(i, j int) bool {
		return ProxySlice[i].AvgSpeed < ProxySlice[j].AvgSpeed
	})
	for _, v := range ProxySlice {
		if v.Status {
			fmt.Printf("节点：%v\n谷歌延迟：%v\n重连次数：%v次\n平均下载速度：%v\n最高下载速度：%v\nIP：%v\n", v.Name, v.Ping, v.Retries, lib.BytesToSize(v.AvgSpeed), lib.BytesToSize(v.MaxSpeed), v.RealIp)
		}
	}
	lib.Log().Info("---------------以上为测速结果，根据平均下载速度[低->高]排序！！！---------------")
	KillProcess()
}

//捕获退出信号
func GetSingle() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	for range sigChan {
		fmt.Printf("\n")
		lib.Log().Warning("接收到退出信号，即将清理进程并退出！！！")
		lib.SignalOut = true
		KillProcess()
		fmt.Printf("\n")
		os.Exit(1)
	}
}

//创建cli说明
func CreateFlag() *Flag {
	var v = new(Flag)
	flag.BoolVar(&v.Help, "h", false, "使用说明")
	flag.StringVar(&v.Subscribe, "u", "", "vmess订阅链接")
	flag.StringVar(&v.Filter, "s", "", "筛选节点，或条件：|，与条件：&")
	flag.IntVar(&v.Duration, "d", 15, "测速文件下载时间，单位/秒，默认15秒")
	flag.Parse()
	if v.Help {
		_, _ = fmt.Fprintf(os.Stderr, `机场测速工具，当前版本：1.0.0

使用说明:
`)
		flag.PrintDefaults()
		os.Exit(0)
	}
	if v.Subscribe == "" {
		if len(flag.Args()) == 0 {
			lib.Log().Error("请加参数 -u 输入订阅链接")
			os.Exit(0)
		}
	}
	return v
}

//清理代理相关内容
func KillProcess() {
	//结束代理进程
	_, _ = exec.Command(commandUse, commandArg, "taskkill", "/f", "/im", "xray.exe").Output()
	_, _ = exec.Command(commandUse, commandArg, "ps aux|grep 'xray'|awk '{print $2}'|xargs kill -9").Output()
	//删除配置文件
	_ = os.RemoveAll("client/config")
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
//创建配置文件
func CreateConfigFile(index int, node *ProxyNode, fileWg *sync.WaitGroup) (err error) {
	defer fileWg.Done()
	name := strings.TrimSpace(fmt.Sprintf("%v", node.Detail["ps"]))
	ProxySlice[index].Name = name
	configDir := fmt.Sprintf("%v/client/config/%v.json", dir, index)
	add := fmt.Sprintf("%v", node.Detail["add"])
	aid := fmt.Sprintf("%v", node.Detail["aid"])
	host := fmt.Sprintf("%v", node.Detail["host"])
	id := fmt.Sprintf("%v", node.Detail["id"])
	net := fmt.Sprintf("%v", node.Detail["net"])
	path := fmt.Sprintf("%v", node.Detail["path"])
	port := fmt.Sprintf("%v", node.Detail["port"])
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
`, 40000+index, add, port, id, aid, net, host, path)
	err = ioutil.WriteFile(configDir, []byte(tmp), 0644)
	if err != nil {
		lib.Log().Error("配置文件创建失败[%v]\n%v", name, err.Error())
		ProxySlice[index].ErrorMessage = err.Error()
		ProxySlice[index].Status = false
		return
	}
	err = ExecProxyCore(fmt.Sprintf("%v/client/config/%v.json", dir, index), name)
	if err != nil {
		ProxySlice[index].ErrorMessage = err.Error()
		ProxySlice[index].Status = false
		return
	}
	ProxySlice[index].Proxy = fmt.Sprintf("socks5://127.0.0.1:%v", 40000+index)
	return
}

//执行代理客户端
func ExecProxyCore(jsonPath string, name string) (err error) {
	cmd := fmt.Sprintf("%v -config=%v", clientPath, jsonPath)
	cmdR := exec.Command(commandUse, commandArg, cmd)
	err = cmdR.Start()
	if err != nil {
		lib.Log().Error("启动[%v]代理客户端出错：%v\n%v", name, err.Error(), cmdR.Args)
		return
	}
	return
}

//测试节点连接情况
func CurlGoogle(index int, node *ProxyNode, googleWg *sync.WaitGroup) {
	defer googleWg.Done()
	r, err := lib.Request("https://www.google.com", node.Proxy, 5*time.Second)
	if err != nil {
		lib.Log().Error("节点连接失败：[%v]\n%v", node.Name, err.Error())
		ProxySlice[index].ErrorMessage = err.Error()
		ProxySlice[index].Status = false
		return
	}
	ipBody, err := lib.Request("https://myip.ipip.net/", r.Proxy, 5*time.Second)
	if err != nil {
		lib.Log().Info("节点连接成功：[%v]，请求次数：%v，耗时：%v，获取IP信息失败：\n%v", node.Name, 6-r.Retries, r.Duration, err.Error())
		ProxySlice[index].ErrorMessage = err.Error()
	} else {
		lib.Log().Info("节点连接成功：[%v]，请求次数：%v，耗时：%v，IP信息：\n%v", node.Name, 6-r.Retries, r.Duration, ipBody.Body)
		ProxySlice[index].RealIp = ipBody.Body
	}
	ProxySlice[index].Ping = r.Duration
	ProxySlice[index].Retries = 6 - r.Retries
	ProxySlice[index].Status = true
}
