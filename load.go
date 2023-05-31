package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
)

const (
	UrlRedisCharge = "https://d80nal86k5.execute-api.us-east-1.amazonaws.com/prod/charge-request-redis"
	UrlRedisReset  = "https://d80nal86k5.execute-api.us-east-1.amazonaws.com/prod/reset-redis"

	UrlMemcachedCharge = "https://njiic2kr0a.execute-api.us-east-1.amazonaws.com/prod/charge-request-memcached"
	UrlMemcachedReset  = "https://njiic2kr0a.execute-api.us-east-1.amazonaws.com/prod/reset-memcached"
)

type backend struct {
	chargeURL string
	resetURL  string
}

var redisBackend = backend{
	chargeURL: UrlRedisCharge,
	resetURL:  UrlRedisReset,
}

var memcachedBackend = backend{
	chargeURL: UrlMemcachedCharge,
	resetURL:  UrlMemcachedReset,
}

type response struct {
	RemainingBalance int  `json:"remainingBalance"`
	Charges          int  `json:"charges"`
	IsAuthorized     bool `json:"isAuthorized"`
}

func main() {
	log.Println("Executing redis tests")
	runRedis()
	log.Println("Executing memcached tests")
	runMemcached()
}

func runRedis() {
	for i := 0; i < 10000; i++ {
		if i%10 == 0 {
			log.Println("iteration", i)
		}
		if err := run(redisBackend); err != nil {
			panic(err)
		}
	}
}

func runMemcached() {
	for i := 0; i < 100; i++ {
		if i%10 == 0 {
			log.Println("iteration", i)
		}
		if err := run(memcachedBackend); err != nil {
			panic(err)
		}
	}
}

func run(b backend) error {
	if resp, err := http.Post(b.resetURL, "", nil); err != nil {
		panic(err)
	} else {
		defer resp.Body.Close()
	}

	wg := sync.WaitGroup{}
	start := sync.WaitGroup{}
	start.Add(21)

	var numAuthorized atomic.Int32
	var negativeBalance atomic.Int32

	for i := 0; i < 21; i++ {
		wg.Add(1)
		go func() {
			start.Done()
			start.Wait()
			defer wg.Done()
			r := charge(b)
			if r.IsAuthorized {
				numAuthorized.Add(1)
			}
			if r.RemainingBalance < 0 {
				negativeBalance.Add(1)
			}
		}()
	}

	wg.Wait()

	if n := numAuthorized.Load(); n != 20 {
		return fmt.Errorf("expected 20 authorized charges, got %d", n)
	}
	if n := negativeBalance.Load(); n != 0 {
		return fmt.Errorf("expected no negative balances, got %d", n)
	}

	return nil
}

func charge(b backend) response {
	resp, err := http.Post(b.chargeURL, "", nil)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != http.StatusOK {
		panic(string(body))
	}

	var r response
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		panic(string(body))
	}
	return r
}
