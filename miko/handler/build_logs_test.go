package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/miko/domain"
	"github.com/Hayao0819/Kamisato/miko/joblog"
	"github.com/Hayao0819/Kamisato/miko/test/mocks"
)

func TestJobLogsHandlerEmitsLinesOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)
	h := New(mockSvc, Settings{MaxLogReaders: 8})

	buf := joblog.New(0)
	_, _ = buf.Write([]byte("line1\nline2\nline3\n"))
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
	synctest.Test(t, func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := mocks.NewMockServicer(ctrl)
		h := New(mockSvc, Settings{MaxLogReaders: 8})

		// A chunk that does not end in a newline must be held, not framed on its own.
		buf := joblog.New(0)
		_, _ = buf.Write([]byte("hello, "))
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

		// Wait until the handler has polled the partial line and is durably blocked
		// on its ticker, holding "hello, " unframed, before the newline arrives.
		synctest.Wait()
		_, _ = buf.Write([]byte("world\n"))
		buf.Close()
		<-done

		body := w.Body.String()
		if c := strings.Count(body, "data: hello, world\n\n"); c != 1 {
			t.Errorf("merged frame appeared %d times, want exactly 1\nbody:\n%s", c, body)
		}
		if strings.Contains(body, "data: hello, \n\n") {
			t.Errorf("partial line was framed before its newline arrived:\n%s", body)
		}
	})
}

func TestJobLogsHandlerReaderCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := mocks.NewMockServicer(ctrl)

	const cap = 3
	h := New(mockSvc, Settings{MaxLogReaders: cap})

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

func TestHandlerDefaultsLogReaderCap(t *testing.T) {
	h := New(nil, Settings{MaxLogReaders: -1})
	if h.settings.MaxLogReaders != 8 {
		t.Fatalf("max log readers = %d, want 8", h.settings.MaxLogReaders)
	}
}

func TestStoredJobLogsSupportPlainTextAndEventSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, logs, accept, wantBody, wantType string
	}{
		{"plain text", "line1\npartial", "", "line1\npartial", "text/plain; charset=utf-8"},
		{"SSE lines and partial tail", "line1\npartial", "text/event-stream", "data: line1\n\ndata: partial\n\n", "text/event-stream"},
		{"SSE final newline", "line1\n", "text/event-stream", "data: line1\n\n", "text/event-stream"},
		{"SSE empty", "", "text/event-stream", "", "text/event-stream"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockSvc := mocks.NewMockServicer(ctrl)
			mockSvc.EXPECT().Status("job1").Return(&domain.BuildJob{ID: "job1", Logs: test.logs}, nil)
			mockSvc.EXPECT().LogBuffer("job1").Return(nil)
			h := New(mockSvc, Settings{})
			engine := gin.New()
			engine.GET("/jobs/:id/logs", h.JobLogsHandler)
			request := httptest.NewRequest(http.MethodGet, "/jobs/job1/logs", nil)
			request.Header.Set("Accept", test.accept)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Header().Get("Content-Type") != test.wantType || response.Body.String() != test.wantBody {
				t.Fatalf("response = %d %q %q, want 200 %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String(), test.wantType, test.wantBody)
			}
		})
	}
}
