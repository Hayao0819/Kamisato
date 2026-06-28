package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// drainPullStream consumes the image-pull progress stream and surfaces any
// error delivered as a JSON message in the body (ImagePull only reports the
// initial request error directly).
func drainPullStream(r io.ReadCloser) error {
	defer r.Close()
	dec := json.NewDecoder(r)
	for {
		var msg struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
	}
}

// syncBuffer is a concurrency-safe bytes.Buffer for the log capture goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
