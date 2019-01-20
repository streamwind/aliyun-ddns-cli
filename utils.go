package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const minTimeout = 2000 * time.Millisecond
//const regxIP = `(25[0-5]|2[0-4]\d|[0-1]\d{2}|[1-9]?\d)\.(25[0-5]|2[0-4]\d|[0-1]\d{2}|[1-9]?\d)\.(25[0-5]|2[0-4]\d|[0-1]\d{2}|[1-9]?\d)\.(25[0-5]|2[0-4]\d|[0-1]\d{2}|[1-9]?\d)`
const regxIP = `([a-f0-9]{1,4}(:[a-f0-9]{1,4}){7}|[a-f0-9]{1,4}(:[a-f0-9]{1,4}){0,7}::[a-f0-9]{0,4}(:[a-f0-9]{1,4}){0,7})`

var ipAPI = []string{
	"http://www.taobao.com/help/getip.php", "http://ddns.oray.com/checkip", "http://haoip.cn",
	"http://cnc.synology.cn:81", "http://jpc.synology.com:81", "http://usc.synology.com:81",
	"http://ip.6655.com/ip.aspx", "http://pv.sohu.com/cityjson?ie=utf-8", "http://whois.pconline.com.cn/ipJson.jsp",
}

var curlVer = []string{
	"7.59.0", "7.58.0", "7.57.0", "7.56.1", "7.56.0", "7.55.1", "7.55.0", "7.54.1", "7.54.0", "7.53.1", "7.53.0", "7.52.1",
	"7.52.0", "7.51.0", "7.50.3", "7.50.2", "7.50.1", "7.50.0", "7.49.1", "7.49.0", "7.48.0", "7.47.1", "7.47.0", "7.46.0",
	"7.45.0", "7.44.0", "7.43.0", "7.42.1", "7.42.0", "7.41.0", "7.40.0", "7.39.0", "7.38.0", "7.37.1", "7.37.0", "7.36.0",
}

func getIP() (ip string) {
	var (
		length   = len(ipAPI)
		ipMap    = make(map[string]int, length)
		cchan    = make(chan string, length)
		regx     = regexp.MustCompile(regxIP)
		maxCount = -1
	)
	for _, url := range ipAPI {
		go func(url string) {
			cchan <- regx.FindString(wGet(url, minTimeout))
		}(url)
	}
	for i := 0; i < length; i++ {
		v := <-cchan
		if 0 == len(v) {
			continue
		}
		if ipMap[v] >= length/2 {
			return v
		}
		ipMap[v]++
	}
	for k, v := range ipMap {
		if v > maxCount {
			maxCount = v
			ip = k
		}
	}

	// Use First ipAPI as failsafe
	if 0 == len(ip) {
		ip = regexp.MustCompile(regxIP).FindString(wGet(ipAPI[0], 5*minTimeout))
	}
	return
}

func wGet(url string, timeout time.Duration) (str string) {
	request, err := http.NewRequest("GET", url, nil)
	request.Header.Set("User-Agent", "curl/"+curlVer[rand.Intn(len(curlVer))])
	if err != nil {
		return
	}
	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(request)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	str = string(body)
	// fmt.Println(url, regexp.MustCompile(regxIP).FindString(str))
	return
}

func getDNS(domain string) (ip string) {
	var (
		maxServerCount = 5
		dnsMap         = make(map[string]int, maxServerCount)
		cchan          = make(chan string, maxServerCount)
		maxCount       = -1
		udpClient      = &dns.Client{Net: "udp", Timeout: time.Second}
	)

	for i := 0; i < maxServerCount; i++ {
		go func(dns string) {
			cchan <- getFisrtARecord(udpClient, dns, domain)
		}(fmt.Sprintf("dns%d.hichina.com:53", rand.Intn(30)+1))
	}

	for i := 0; i < maxServerCount; i++ {
		v := <-cchan
		if len(v) == 0 {
			continue
		}
		if dnsMap[v] >= maxServerCount/2 {
			return v
		}
		dnsMap[v]++
	}

	for k, v := range dnsMap {
		if v > maxCount {
			maxCount = v
			ip = k
		}
	}
	return
}

func getFisrtARecord(client *dns.Client, dnsServer, targetDomain string) (ip string) {
	if !strings.HasSuffix(targetDomain, ".") {
		targetDomain += "."
	}
	msg := new(dns.Msg)
	msg.SetQuestion(targetDomain, dns.TypeA)
	r, _, err := client.Exchange(msg, dnsServer)
	if err != nil && (r == nil || r.Rcode != dns.RcodeSuccess) {
		return
	}
	for _, rr := range r.Answer {
		if a, ok := rr.(*dns.A); ok {
			ip = a.A.String()
			break
		}
	}
	return
}
