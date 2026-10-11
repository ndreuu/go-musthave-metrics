package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunHTTPServerWaitsForActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
	})
	server := &http.Server{
		Addr: address,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
			_, _ = io.WriteString(w, "saved")
		}),
	}
	stopped := make(chan error, 1)
	go func() { stopped <- runHTTPServer(ctx, server) }()

	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		deadline := time.Now().Add(3 * time.Second)
		for {
			response, err := client.Get("http://" + address)
			if err != nil {
				if time.Now().Before(deadline) {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				requestDone <- err
				return
			}
			_, err = io.Copy(io.Discard, response.Body)
			closeErr := response.Body.Close()
			if err == nil {
				err = closeErr
			}
			requestDone <- err
			return
		}
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not reach the server")
	}
	cancel()
	select {
	case err := <-stopped:
		t.Fatalf("server stopped before the active request finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop after the request finished")
	}
	if err := <-requestDone; err != nil {
		t.Fatalf("active request failed during shutdown: %v", err)
	}
}

func TestRunHTTPServerReturnsStartupError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	server := &http.Server{Addr: listener.Addr().String()}
	stopped := make(chan error, 1)
	go func() { stopped <- runHTTPServer(context.Background(), server) }()
	select {
	case err := <-stopped:
		if err == nil || !strings.Contains(err.Error(), "serve HTTP") {
			t.Fatalf("expected a server startup error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("startup error did not stop the server lifecycle")
	}
}
