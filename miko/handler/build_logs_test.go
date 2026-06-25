package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/joblog"
	"github.com/Hayao0819/Kamisato/miko/test/mocks"
	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

// TestJobLogsHandlerEmitsLinesOnce streams a closed buffer and asserts each line
// is emitted exactly once as an SSE data frame.
func TestJobLogsHandlerEmitsLinesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)
	h := New(mockSvc, &conf.MikoConfig{MaxLogReaders: 8})

	buf := joblog.New(0)
	buf.Write([]byte("line1\nline2\nline3\n"))
	buf.Close()
	job := &domain.BuildJob{ID: "job1", Log: buf}
	mockSvc.EXPECT().Status("job1").Return(job, nil)

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

// TestJobLogsHandlerReaderCap asserts the (N+1)th concurrent reader of one job
// gets 429. The first N readers are simulated by pre-incrementing the counter.
func TestJobLogsHandlerReaderCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)

	const cap = 3
	h := New(mockSvc, &conf.MikoConfig{MaxLogReaders: cap})

	job := &domain.BuildJob{ID: "job1", Log: joblog.New(0)}
	mockSvc.EXPECT().Status("job1").Return(job, nil)

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
