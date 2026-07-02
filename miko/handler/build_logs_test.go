package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/joblog"
	"github.com/Hayao0819/Kamisato/miko/test/mocks"
	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func TestJobLogsHandlerEmitsLinesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)
	h := New(mockSvc, &conf.MikoConfig{MaxLogReaders: 8})

	buf := joblog.New(0)
	buf.Write([]byte("line1\nline2\nline3\n"))
	buf.Close()
	mockSvc.EXPECT().Status("job1").Return(&domain.BuildJob{ID: "job1"}, nil)
	mockSvc.EXPECT().LogBuffer("job1").Return(buf)

	r := gin.New()
	r.GET("/jobs/:id/logs", h.JobLogsHandler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/jobs/job1/logs", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"data: line1\n\n", "data: line2\n\n", "data: line3\n\n"} {
		if c := strings.Count(body, want); c != 1 {
			t.Errorf("frame %q appeared %d times, want exactly 1\nbody:\n%s", want, c, body)
		}
	}
	// No empty trailing data frame from the final newline.
	if strings.Contains(body, "data: \n\n") {
		t.Errorf("emitted an empty trailing data frame:\n%s", body)
	}
}

func TestJobLogsHandlerHoldsPartialLine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)
	h := New(mockSvc, &conf.MikoConfig{MaxLogReaders: 8})

	// A chunk that does not end in a newline must be held, not framed on its own.
	buf := joblog.New(0)
	buf.Write([]byte("hello, "))
	mockSvc.EXPECT().Status("job1").Return(&domain.BuildJob{ID: "job1"}, nil)
	mockSvc.EXPECT().LogBuffer("job1").Return(buf)

	r := gin.New()
	r.GET("/jobs/:id/logs", h.JobLogsHandler)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/jobs/job1/logs", nil))
		close(done)
	}()

	// Let the handler poll the partial line on its own before the newline arrives.
	time.Sleep(300 * time.Millisecond)
	buf.Write([]byte("world\n"))
	buf.Close()
	<-done

	body := w.Body.String()
	if c := strings.Count(body, "data: hello, world\n\n"); c != 1 {
		t.Errorf("merged frame appeared %d times, want exactly 1\nbody:\n%s", c, body)
	}
	if strings.Contains(body, "data: hello, \n\n") {
		t.Errorf("partial line was framed before its newline arrived:\n%s", body)
	}
}

func TestJobLogsHandlerReaderCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)

	const cap = 3
	h := New(mockSvc, &conf.MikoConfig{MaxLogReaders: cap})

	// The reader cap rejects with 429 before the live buffer is ever consulted.
	mockSvc.EXPECT().Status("job1").Return(&domain.BuildJob{ID: "job1"}, nil)

	// Simulate cap readers already streaming this job.
	h.logReadersMu.Lock()
	h.logReaders["job1"] = cap
	h.logReadersMu.Unlock()

	r := gin.New()
	r.GET("/jobs/:id/logs", h.JobLogsHandler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/jobs/job1/logs", nil))

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", w.Code, w.Body.String())
	}
	// The counter must not have been bumped past cap by the rejected request.
	h.logReadersMu.Lock()
	got := h.logReaders["job1"]
	h.logReadersMu.Unlock()
	if got != cap {
		t.Errorf("reader count = %d, want unchanged %d after 429", got, cap)
	}
}
