package proxymanager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/thoas/go-funk"
	"ktbs.dev/mubeng/pkg/helper"
	"ktbs.dev/mubeng/pkg/mubeng"
)

// ProxyManager defines the proxy list and current proxy position
type ProxyManager struct {
	CurrentIndex int
	filepath     string
	Length       int
	Proxies      []string
	Remote       bool
}

func init() {
	rand.Seed(time.Now().UnixNano())

	manager = &ProxyManager{CurrentIndex: -1}
}

type ProxyItem struct {
	Anonymous  string `json:"anonymous"`
	CheckCount int    `json:"check_count"`
	FailCount  int    `json:"fail_count"`
	Https      bool   `json:"https"`
	LastStatus bool   `json:"last_status"`
	LastTime   string `json:"last_time"`
	Protocol   string `json:"protocol"`
	Proxy      string `json:"proxy"`
	Region     string `json:"region"`
	Source     string `json:"source"`
}

// New initialize ProxyManager
func New(filename string, remote bool) (*ProxyManager, error) {
	keys := make(map[string]bool)

	if remote {
		resp, err := http.Get(filename)
		if err != nil {
			return nil, fmt.Errorf("get %s remote proxy list fail: %s", filename, err)
		}
		defer resp.Body.Close()
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response body fail: %s", err)
		}
		var proxies []*ProxyItem
		err = json.Unmarshal(content, &proxies)
		if err != nil {
			return nil, fmt.Errorf("unmarshal proxy response fail: %s", err)
		}

		sort.Slice(proxies, func(i, j int) bool {
			return proxies[i].CheckCount > proxies[j].CheckCount
		})

		for _, item := range proxies {
			keys[fmt.Sprintf("%s://%s", item.Protocol, item.Proxy)] = true
		}

		manager.Proxies = append(manager.Proxies, funk.UniqString(funk.Keys(keys).([]string))...)

	} else {
		file, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		manager.Proxies = []string{}
		manager.filepath = filename

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			proxy := helper.Eval(scanner.Text())
			if _, value := keys[proxy]; !value {
				_, err = mubeng.Transport(placeholder.ReplaceAllString(proxy, ""))
				if err == nil {
					keys[proxy] = true
					manager.Proxies = append(manager.Proxies, proxy)
				}
			}
		}
	}

	manager.Length = len(manager.Proxies)
	if manager.Length < 1 {
		return manager, fmt.Errorf("open %s: has no valid proxy URLs", filename)
	}

	manager.Remote = remote
	return manager, nil
}
