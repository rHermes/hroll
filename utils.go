package main

import (
	"bytes"
	srand "crypto/rand"
	"encoding/hex"
	"io"
	"math/rand"
	"net/url"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func pInt64(x int64) *int64 {
	return &x
}

func genRandomName() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func genSecureRandomName() string {
	var buf bytes.Buffer
	if _, err := io.CopyN(hex.NewEncoder(&buf), srand.Reader, 16); err != nil {
		panic(err)
	}
	return buf.String()
}

func mustParseURL(u string) *url.URL {
	if ul, err := url.Parse(u); err != nil {
		panic(err)
	} else {
		return ul
	}
}

func getLogger() (*zap.Logger, error) {
	conf := zap.NewDevelopmentConfig()
	conf.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return conf.Build()
}
