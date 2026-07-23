package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestSubPilotRuntimeConfigRefreshIsCoalesced(t *testing.T) {
	var configCalls atomic.Int64
	var selectCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case subPilotRuntimeConfigPath:
			configCalls.Add(1)
			time.Sleep(30 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(subPilotRuntimeConfig{
				DispatchEnabled: true, DispatchFailOpen: true, DispatchSelectTimeoutMS: 200,
				DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
			})
		case subPilotSelectPath:
			selectCalls.Add(1)
			_, _ = w.Write([]byte(`{"decision":"selected","account":{"id":"1"},"lease":{"id":"lease-1"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	client := newSubPilotClient(cfg)
	var wg sync.WaitGroup
	for index := 0; index < 64; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.selectAccount(context.Background(), subPilotSelectRequest{
				RequestID: "req", Platform: "openai", Model: "gpt-test",
			})
		}()
	}
	wg.Wait()
	if configCalls.Load() != 1 {
		t.Fatalf("runtime config calls = %d, want 1", configCalls.Load())
	}
	if selectCalls.Load() != 64 {
		t.Fatalf("select calls = %d, want 64", selectCalls.Load())
	}
}

func TestSubPilotSharedHTTPClientKeepsEnoughIdleConnections(t *testing.T) {
	client := newSubPilotSharedHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	if transport.MaxIdleConns != subPilotMaxIdleConns || transport.MaxIdleConnsPerHost != subPilotMaxIdleConns {
		t.Fatalf("idle connection limits = %d/%d, want %d/%d",
			transport.MaxIdleConns, transport.MaxIdleConnsPerHost,
			subPilotMaxIdleConns, subPilotMaxIdleConns,
		)
	}
}

func TestCleanupSubPilotLeasesRemovesOnlyExpiredEntries(t *testing.T) {
	now := time.Now()
	expiredKey := "expired|1"
	currentKey := "current|1"
	subPilotLeases.Store(expiredKey, subPilotLeaseRecord{LeaseID: "old", CreatedAt: now.Add(-2 * subPilotLeaseMaxAge)})
	subPilotLeases.Store(currentKey, subPilotLeaseRecord{LeaseID: "new", CreatedAt: now})
	t.Cleanup(func() {
		subPilotLeases.Delete(expiredKey)
		subPilotLeases.Delete(currentKey)
	})

	cleanupSubPilotLeases(now, subPilotLeaseMaxAge)
	if _, ok := subPilotLeases.Load(expiredKey); ok {
		t.Fatal("expired lease was not removed")
	}
	if _, ok := subPilotLeases.Load(currentKey); !ok {
		t.Fatal("current lease was removed")
	}
}

func TestSubPilotReportEnqueueDoesNotWaitForNetwork(t *testing.T) {
	called := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != subPilotReportSuccessPath {
			http.NotFound(w, r)
			return
		}
		time.Sleep(150 * time.Millisecond)
		called <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	client := newSubPilotClient(cfg)
	started := time.Now()
	client.reportSuccess(context.Background(), subPilotReportSuccessRequest{RequestID: "req", AccountID: "1"})
	if elapsed := time.Since(started); elapsed > 20*time.Millisecond {
		t.Fatalf("report enqueue took %s", elapsed)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("queued report was not delivered")
	}
}
