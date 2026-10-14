package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-musthave-metrics/internal/agent"
	serverconfig "go-musthave-metrics/internal/config/server"
	"go-musthave-metrics/internal/handler"
	"go-musthave-metrics/internal/repository"
	"go-musthave-metrics/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func testRouter(t *testing.T, cfg *serverconfig.Config) (*gin.Engine, *repository.MemStorage) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	storage := repository.NewMemStorage("")
	metricsHandler := handler.NewMetricsHandler(service.NewMetricsService(storage), storage, "", cfg.Key, nil)
	router, err := newRouter(cfg, zap.NewNop(), nil, metricsHandler, nil)
	if err != nil {
		t.Fatal(err)
	}
	return router, storage
}

func TestRouterTrustedSubnetUpdateRoutes(t *testing.T) {
	routes := []struct {
		path string
		body string
	}{
		{"/update/gauge/Metric/2.5", ""},
		{"/update", `{"id":"Metric","type":"gauge","value":2.5}`},
		{"/update/", `{"id":"Metric","type":"gauge","value":2.5}`},
		{"/updates", `[{"id":"Metric","type":"gauge","value":2.5}]`},
		{"/updates/", `[{"id":"Metric","type":"gauge","value":2.5}]`},
	}
	cases := []struct {
		name   string
		subnet string
		ip     string
		status int
	}{
		{"trusted IPv4", "192.168.1.0/24", "192.168.1.42", http.StatusOK},
		{"untrusted IPv4", "192.168.1.0/24", "192.168.2.42", http.StatusForbidden},
		{"trusted IPv6", "2001:db8::/32", "2001:db8:1::42", http.StatusOK},
		{"missing header", "192.168.1.0/24", "", http.StatusForbidden},
		{"invalid header", "192.168.1.0/24", "not-an-ip", http.StatusForbidden},
		{"disabled without header", "", "", http.StatusOK},
		{"disabled with invalid header", "", "not-an-ip", http.StatusOK},
	}
	for _, route := range routes {
		for _, tc := range cases {
			t.Run(route.path+"/"+tc.name, func(t *testing.T) {
				router, storage := testRouter(t, &serverconfig.Config{TrustedSubnet: tc.subnet})
				request := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
				request.Header.Set("Content-Type", "application/json")
				if tc.ip != "" {
					request.Header.Set("X-Real-IP", tc.ip)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if response.Code != tc.status {
					t.Fatalf("status = %d, want %d; body = %s", response.Code, tc.status, response.Body)
				}
				if tc.status == http.StatusOK {
					value, err := storage.GetGauge(context.Background(), "Metric")
					if err != nil || value != 2.5 {
						t.Fatalf("stored metric = %v, error = %v; want 2.5", value, err)
					}
				} else if len(storage.GetAll()) != 0 {
					t.Fatal("rejected request modified storage")
				}
			})
		}
	}
}

func TestRouterTrustedSubnetBeforeBodyProcessing(t *testing.T) {
	router, storage := testRouter(t, &serverconfig.Config{TrustedSubnet: "192.168.1.0/24", Key: "secret"})
	request := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader("invalid gzip"))
	request.Header.Set("X-Real-IP", "192.168.2.42")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("HashSHA256", "invalid hash")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || len(storage.GetAll()) != 0 {
		t.Fatalf("status = %d, metrics = %v; want 403 and no metrics", response.Code, storage.GetAll())
	}
}

func TestRouterTrustedSubnetReadRoutes(t *testing.T) {
	router, storage := testRouter(t, &serverconfig.Config{TrustedSubnet: "192.168.1.0/24"})
	if err := storage.SetGauge(context.Background(), "Metric", 2.5); err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/", ""},
		{http.MethodGet, "/value/gauge/Metric", ""},
		{http.MethodPost, "/value", `{"id":"Metric","type":"gauge"}`},
		{http.MethodPost, "/value/", `{"id":"Metric","type":"gauge"}`},
	} {
		t.Run(route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body)
			}
		})
	}
}

func TestAgentSendsToTrustedSubnetServer(t *testing.T) {
	for _, subnet := range []string{"127.0.0.0/8", "192.0.2.0/24"} {
		t.Run(subnet, func(t *testing.T) {
			cfg := &serverconfig.Config{TrustedSubnet: subnet, Key: "secret"}
			router, storage := testRouter(t, cfg)
			server := httptest.NewServer(router)
			defer server.Close()
			sender := agent.NewSender(server.URL, cfg.Key, "")
			metric := &agent.Metric{MType: "gauge", Name: "Single", Value: 2.5}
			for _, send := range []func() error{
				func() error { return sender.Send(metric) },
				func() error {
					return sender.SendBatch([]*agent.Metric{{MType: "counter", Name: "Batch", Value: 3}})
				},
			} {
				err := send()
				if subnet == "127.0.0.0/8" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "status 403") {
					t.Fatalf("send error = %v, want status 403", err)
				}
			}
			if subnet == "127.0.0.0/8" {
				gauge, gaugeErr := storage.GetGauge(context.Background(), "Single")
				counter, counterErr := storage.GetCounter(context.Background(), "Batch")
				if gaugeErr != nil || counterErr != nil || gauge != 2.5 || counter != 3 {
					t.Fatalf("stored gauge = %v (%v), counter = %v (%v)", gauge, gaugeErr, counter, counterErr)
				}
			} else if len(storage.GetAll()) != 0 {
				t.Fatal("rejected agent modified storage")
			}
		})
	}
}
