package main

import (
	"net/http"
	"os"
	"time"
)

func hasHealthcheck(args []string) bool {
	for _, a := range args {
		if a == "-healthcheck" {
			return true
		}
	}
	return false
}

func runHealthcheck(port string) {
	_ = port
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get("http://127.0.0.1:8080/healthz")
	if err != nil {
		os.Exit(1)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	os.Exit(0)
}
