package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

type FireXProxy struct {
	Server     string  `json:"server"`
	Port       int     `json:"port"`
	IsoCode    string  `json:"iso_code"`
	Country    string  `json:"country"`
	Protocol   string  `json:"protocol"`
	PingTimeMs int     `json:"ping_time_ms"`
	LossRatio  float64 `json:"loss_ratio"`
}

func getFireXProxies() ([]FireXProxy, error) {
	resp, err := http.Get("https://api.firexproxy.com/v1/proxy")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)

	var proxies []FireXProxy
	jdec := json.NewDecoder(resp.Body)
	if err := jdec.Decode(&proxies); err != nil {
		return nil, err
	}
	return proxies, nil
}

func filterFireXProxies(proxies []FireXProxy, ptype string) []FireXProxy {
	nw := make([]FireXProxy, 0)
	for _, p := range proxies {
		if p.Protocol == ptype {
			nw = append(nw, p)
		}
	}
	return nw
}
