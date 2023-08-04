package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"time"
)

func main() {
	// TODO: maybe add some json serialization
	msg := []byte("hello bench.\n")
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(msg)
	}))
	defer ts.Close()

	tr := &http.Transport{}
	tr.MaxIdleConnsPerHost = 100
	cl := &http.Client{
		Transport: tr,
	}

	port := "8080"
	if len(os.Getenv("PORT")) != 0 {
		port = os.Getenv("PORT")
	}

	http.ListenAndServe("0.0.0.0:"+port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		iterations := r.URL.Query().Get("iterations")
		n, err := strconv.ParseInt(iterations, 10, 64)
		if err != nil {
			n = 100000
		}

		parallel := r.URL.Query().Get("parallel")
		p, err := strconv.ParseInt(parallel, 10, 64)
		if err != nil {
			n = 8
		}

		if n <= p {
			http.Error(w, "iterations have to be bigger than parallel", http.StatusBadRequest)
			return
		}

		if n == 0 || p == 0 {
			http.Error(w, "parallel/iterations have to be at least 1", http.StatusBadRequest)
			return
		}

		startMsg := fmt.Sprintf("running bench with %d iterations and %d in parallel", n, p)
		log.Println(startMsg)
		fmt.Fprintf(w, "%s\n", startMsg)

		res := runBench(cl, ts.URL, msg, int(n), int(p))

		log.Println(res)
		fmt.Fprintf(w, "%s\n", res)
	}))
}

func runBench(client *http.Client, url string, msg []byte, iterations, parallel int) string {
	before := time.Now()

	sem := make(chan struct{}, parallel)
	for i := 0; i < iterations; i++ {
		sem <- struct{}{}
		go func() {
			if err := doReq(client, url, msg); err != nil {
				log.Println("error during bench: %s", err)
			}
			<-sem
		}()
	}

	return fmt.Sprintf("bench took: %s, %d ns/op", time.Since(before), time.Since(before).Nanoseconds()/int64(iterations))
}

func doReq(client *http.Client, url string, msg []byte) error {
	res, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("error during GET: %w", err)
	}
	all, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	if !bytes.Equal(all, msg) {
		return fmt.Errorf("got body %q; want %q", all, msg)
	}

	return nil
}
