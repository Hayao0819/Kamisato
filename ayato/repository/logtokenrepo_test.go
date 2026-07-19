package repository

import (
	"sync"
	"testing"
	"time"
)

func TestLogTokenConsumeIsAtomic(t *testing.T) {
	repository := NewLogTokenRepository(newTestKV(t))
	token, err := repository.Mint("job-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	const contenders = 16
	start := make(chan struct{})
	results := make(chan bool, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			jobID, ok, consumeErr := repository.ConsumeLogToken(token)
			results <- consumeErr == nil && ok && jobID == "job-1"
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	winners := 0
	for won := range results {
		if won {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("successful consumers = %d, want exactly 1", winners)
	}
}
