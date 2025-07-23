package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/csullivannet/rootly-proxy/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) FindByHostname(hostname string) (*database.StatusPage, error) {
	args := m.Called(hostname)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.StatusPage), args.Error(1)
}

type mockHTTPClient struct {
	mock.Mock
}

func (m *mockHTTPClient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestProxyHandler(t *testing.T) {
	tests := []struct {
		name           string
		host           string
		setupRepo      func(upstreamURL string) *mockRepository
		setupHTTP      func() *httptest.Server
		wantStatusCode int
		wantBody       string
	}{
		{
			name: "successful proxy",
			host: "status.acme.com",
			setupRepo: func(upstreamURL string) *mockRepository {
				repo := new(mockRepository)
				repo.On("FindByHostname", "status.acme.com").Return(
					&database.StatusPage{
						ID:          1,
						Hostname:    "status.acme.com",
						PageDataURL: upstreamURL,
					}, nil)
				return repo
			},
			setupHTTP: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("ACME Status Page"))
				}))
			},
			wantStatusCode: http.StatusOK,
			wantBody:       "ACME Status Page",
		},
		{
			name: "unknown domain returns 404",
			host: "unknown.com",
			setupRepo: func(upstreamURL string) *mockRepository {
				repo := new(mockRepository)
				repo.On("FindByHostname", "unknown.com").Return(nil, nil)
				return repo
			},
			setupHTTP: func() *httptest.Server {
				return nil
			},
			wantStatusCode: http.StatusNotFound,
			wantBody:       "404 - Domain not found\n",
		},
		{
			name: "upstream fetch failure returns 502",
			host: "status.error.com",
			setupRepo: func(upstreamURL string) *mockRepository {
				repo := new(mockRepository)
				repo.On("FindByHostname", "status.error.com").Return(
					&database.StatusPage{
						ID:          2,
						Hostname:    "status.error.com",
						PageDataURL: upstreamURL,
					}, nil)
				return repo
			},
			setupHTTP: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					panic("simulated server error")
				}))
			},
			wantStatusCode: http.StatusBadGateway,
			wantBody:       "502 - Bad Gateway\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var upstreamServer *httptest.Server
			if tt.setupHTTP != nil {
				upstreamServer = tt.setupHTTP()
				if upstreamServer != nil {
					defer upstreamServer.Close()
				}
			}

			upstreamURL := ""
			if upstreamServer != nil {
				upstreamURL = upstreamServer.URL
			}

			repo := tt.setupRepo(upstreamURL)
			handler := &ProxyHandler{
				repository: repo,
				httpClient: &http.Client{},
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}
