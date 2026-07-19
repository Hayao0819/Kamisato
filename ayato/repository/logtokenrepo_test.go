package repository

import (
	"sync"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/repository/kv/badgerkv"
)

func TestLogTokenConsumeIsAtomic(t *testing.T) {
	store, err := badgerkv.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repository := NewLogTokenRepository(store)
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
