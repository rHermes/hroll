package main

import (
	"encoding/hex"
	"math/rand"
	"net/url"
)

func pInt64(x int64) *int64 {
	return &x
}

func genRandomName() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func mustParseUrl(u string) *url.URL {
	if ul, err := url.Parse(u); err != nil {
		panic(err)
	} else {
		return ul
	}
}
