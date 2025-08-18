package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TODO: add timeout on http request
// TODO: add logger to application
// TODO: add new functions to the pipeline
// TODO: add collection of query execution time metrics

type Result struct {
	res int
	err error
}

func fetch(url string) Result {
	time.Sleep(2 * time.Second) // long work emission

	resp, err := http.Get(url)
	if err != nil {
		return Result{0, err}
	}

	return Result{resp.StatusCode, err}
}

func generator(ctx context.Context, urls ...string) chan Result {
	result := make(chan Result)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 5)

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			select {
			case <-ctx.Done():
				return
			case result <- fetch(url):

			}
		}(url)
	}

	go func() {
		wg.Wait()
		close(result)
		close(sem)
	}()

	return result
}

func adder(ctx context.Context, ch chan Result) chan Result {
	result := make(chan Result)

	go func() {
		defer close(result)
		for v := range ch {
			select {
			case <-ctx.Done():
				return
			case result <- Result{v.res + 100, v.err}:
			}
		}
	}()

	return result
}

func main() {
	urls := []string{"https://google.com", "https://httpstat.us/400", "https://google.com", "https://httpstat.us/400", "https://google.com", "https://httpstat.us/400"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for v := range adder(ctx, generator(ctx, urls...)) {
		if v.err != nil {
			fmt.Println(v.err.Error())
			continue
		}
		fmt.Println(v)
	}
}
