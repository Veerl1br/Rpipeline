package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	rpipeline "github.com/Veerl1br/Rpipeline"
	"github.com/Veerl1br/Rpipeline/internal/export"
	"github.com/Veerl1br/Rpipeline/internal/fetch"
	"github.com/Veerl1br/Rpipeline/internal/security"
)

func generator(ctx context.Context, urls ...string) chan rpipeline.Result {
	result := make(chan rpipeline.Result)
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
			case result <- fetch.Fetch(ctx, url):

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

func main() {
	urls := []string{"https://google.com", "https://httpstat.us/400", "https://youtube.com", "https://twitch.tv"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := security.CheckSecurity(ctx, generator(ctx, urls...))

	var allResults []rpipeline.Result

	for v := range results {
		if v.Err != nil {
			fmt.Println(v.Err.Error())
			continue
		}
		allResults = append(allResults, v)
		fmt.Println(v)
	}

	if err := export.ExportJSON(allResults); err != nil {
		fmt.Printf("JSON export error: %v\n", err)
	}
}
