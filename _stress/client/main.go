package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/siegeai/siegelistener/_stress/fake"
)

func main() {
	//buf := &bytes.Buffer{}
	//call(buf)

	for i := 0; i < 9; i++ {
		go caller()
	}
	caller()
}

func caller() {
	buf := &bytes.Buffer{}
	for {
		buf.Reset()
		call(buf)
	}
}

func call(buf *bytes.Buffer) {
	obj := fake.JSON()
	if err := json.NewEncoder(buf).Encode(&obj); err != nil {
		panic(err)
	}

	path := fake.String(48)
	url := fmt.Sprintf("http://localhost:8080/%s", path)

	buf2 := bytes.Buffer{}
	gz := gzip.NewWriter(&buf2)
	_, _ = gz.Write(buf.Bytes())
	_ = gz.Close()

	req, err := http.NewRequest(http.MethodPost, url, &buf2)
	if err != nil {
		panic(err)
	}
	//req.Close = true
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	//res, err := client.Do(req)
	res, err := http.DefaultClient.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(io.Discard, res.Body)
	slog.Info("completed request", "url", url)

	time.Sleep(10 * time.Millisecond)
}
