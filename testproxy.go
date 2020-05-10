package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/haproxytech/models/v2"
	"go.uber.org/zap"
)

type ProxyTestResult struct {
	Failed     bool
	StatusCode int
	TotalDur   time.Duration
}

func testProxy(logger *zap.Logger, srv *models.Server, target *url.URL) (*ProxyTestResult, error) {
	prox, err := url.Parse(fmt.Sprintf("socks5://%s:%d", srv.Address, *srv.Port))
	if err != nil {
		return nil, err
	}

	tra := &http.Transport{
		Proxy: http.ProxyURL(prox),
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     true,
	}
	cl := &http.Client{
		Transport: tra,
		Timeout:   5 * time.Second,
	}
	defer cl.CloseIdleConnections()

	// The test result
	ans := &ProxyTestResult{}
	// implement the tracing later
	started := time.Now()
	resp, err := cl.Get(target.String())
	if err != nil {
		var nerr net.Error
		if errors.As(err, &nerr) && nerr.Timeout() {
			ans.Failed = true
			ans.TotalDur = time.Hour * 24
			return ans, nil
		} else {
			return nil, err
		}
	}
	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	// we need to read all
	io.Copy(ioutil.Discard, resp.Body)

	dur := time.Since(started)
	ans.StatusCode = resp.StatusCode
	ans.TotalDur = dur
	ans.Failed = false

	return ans, nil
}
