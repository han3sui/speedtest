package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func main() {
	r := client("https://sub.wild233.cf/link/20D8L4YfgsoR9WeB?sub=3")
	body, _ := ioutil.ReadAll(r.Body)
	str := string(body)
	base64Decode, _ := base64.StdEncoding.DecodeString(str)
	base64DecodeStr := string(base64Decode)
	base64DecodeSlice := strings.Split(base64DecodeStr, "\n")
	fmt.Println(base64DecodeSlice)
}

func client(url string) (r *http.Response) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	request.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36")
	r, err = client.Do(request)
	if err != nil {
		panic(err)
	}
	return r
	//url:="https://sub.wild233.cf/link/20D8L4YfgsoR9WeB?sub=3"
}
