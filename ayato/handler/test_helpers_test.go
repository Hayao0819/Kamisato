package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
)

const (
	testSecret  = "0123456789abcdef0123456789abcdef" // 32 bytes
	testAdminID = int64(42)
)

func setup(t *testing.T) (*gomock.Controller, *mocks.MockServicer, *Set) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	controller := gomock.NewController(t)
	service := mocks.NewMockServicer(controller)
	return controller, service, New(service, nil)
}

func postJSON(
	t *testing.T,
	path, body string,
	handler gin.HandlerFunc,
) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST(path, handler)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}

func expectRepositoryFile(
	service *mocks.MockServicer,
	repo, arch, name, body string,
	meta domain.FileMeta,
	times int,
) {
	service.EXPECT().SignedURL(repo, arch, name).Return("", nil).Times(times)
	service.EXPECT().GetFileWithMeta(repo, arch, name).DoAndReturn(
		func(_, _, _ string) (platform.File, domain.FileMeta, error) {
			file := platform.NewFileStream(
				name,
				"application/octet-stream",
				bufferToReadSeekCloser(bytes.NewBufferString(body)),
			)
			return file, meta, nil
		},
	).Times(times)
}

var errTest = &testError{"boom"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
