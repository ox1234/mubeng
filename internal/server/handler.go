package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
	"ktbs.dev/mubeng/common"
	"ktbs.dev/mubeng/pkg/helper"
	"ktbs.dev/mubeng/pkg/mubeng"
)

// onRequest handles client request
func (p *Proxy) onRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if p.Options.Sync {
		mutex.Lock()
		defer mutex.Unlock()
	}

	resChan := make(chan interface{})

	go func() {
		if (req.URL.Scheme != "http") && (req.URL.Scheme != "https") {
			resChan <- fmt.Errorf("unsupported protocol scheme: %s", req.URL.Scheme)
			return
		}

		log.Debugf("%s %s %s", req.RemoteAddr, req.Method, req.URL)

		for _, proxy := range p.Options.ProxyManager.Proxies {
			log.Infof("try request %s with %s", req.URL.String(), proxy)
			resp, err := p.doRequestWithProxy(req, helper.EvalFunc(proxy))
			if err != nil {
				log.Warnf("do %s through %s request fail: %s", req.URL.String(), proxy, err)
				continue
			}
			if resp.StatusCode == 501 {
				log.Warnf("proxy return not valid")
				continue
			}

			resChan <- resp
			return
		}

		resp, err := p.doRequestWithProxy(req, "")
		if err != nil {
			resChan <- fmt.Errorf("no proxy can request %s url", req.URL.String())
			return
		}
		resChan <- resp
	}()

	var resp *http.Response

	res := <-resChan
	switch res := res.(type) {
	case *http.Response:
		resp = res
		log.Debug(req.RemoteAddr, " ", resp.Status)
	case error:
		err := res
		log.Errorf("%s %s", req.RemoteAddr, err)
		resp = goproxy.NewResponse(req, mime, http.StatusBadGateway, "Proxy server error")
	}

	return req, resp
}

func (p *Proxy) doRequestWithProxy(req *http.Request, rotate string) (*http.Response, error) {
	var client *http.Client
	if rotate == "" {
		proxy := &mubeng.Proxy{}
		client, req = proxy.New(req)
	} else {
		tr, err := mubeng.Transport(rotate)
		if err != nil {
			return nil, fmt.Errorf("construct transport fail: %s", err)
		}
		proxy := &mubeng.Proxy{
			Address:   rotate,
			Transport: tr,
		}
		client, req = proxy.New(req)
		if p.Options.Verbose {
			client.Transport = dump.RoundTripper(tr)
		}
	}

	client.Timeout = p.Options.Timeout

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request fail: %s", err)
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body fail: %s", err)
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(buf))
	return resp, nil
}

// onConnect handles CONNECT method
func (p *Proxy) onConnect(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
	if p.Options.Auth != "" {
		auth := ctx.Req.Header.Get("Proxy-Authorization")
		if auth != "" {
			creds := strings.SplitN(auth, " ", 2)
			if len(creds) != 2 {
				return goproxy.RejectConnect, host
			}

			auth, err := base64.StdEncoding.DecodeString(creds[1])
			if err != nil {
				log.Warnf("%s: Error decoding proxy authorization", ctx.Req.RemoteAddr)
				return goproxy.RejectConnect, host
			}

			if string(auth) != p.Options.Auth {
				log.Errorf("%s: Invalid proxy authorization", ctx.Req.RemoteAddr)
				return goproxy.RejectConnect, host
			}
		} else {
			log.Warnf("%s: Unathorized proxy request to %s", ctx.Req.RemoteAddr, host)
			return goproxy.RejectConnect, host
		}
	}

	return goproxy.MitmConnect, host
}

// onResponse handles backend responses, and removing hop-by-hop headers
func (p *Proxy) onResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	for _, h := range mubeng.HopHeaders {
		resp.Header.Del(h)
	}

	return resp
}

// nonProxy handles non-proxy requests
func nonProxy(w http.ResponseWriter, req *http.Request) {
	if common.Version != "" {
		w.Header().Add("X-Mubeng-Version", common.Version)
	}

	if req.URL.Path == "/cert" {
		w.Header().Add("Content-Type", "application/octet-stream")
		w.Header().Add("Content-Disposition", fmt.Sprint("attachment; filename=", "goproxy-cacert.der"))
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write(goproxy.GoproxyCa.Certificate[0]); err != nil {
			http.Error(w, "Failed to get proxy certificate authority.", 500)
			log.Errorf("%s %s %s %s", req.RemoteAddr, req.Method, req.URL, err.Error())
		}

		return
	}

	http.Error(w, "This is a mubeng proxy server. Does not respond to non-proxy requests.", 500)
}
