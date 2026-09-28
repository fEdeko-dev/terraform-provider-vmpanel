package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestErrorFormats(t *testing.T) {
	if got := errorMessage([]byte(`{"error":{"code":404,"msg":"Хост не найден"}}`)); got != "Хост не найден" {
		t.Fatalf("совместимый формат: %q", got)
	}
	if got := errorMessage([]byte(`{"error":"not_found","message":"Не найдено"}`)); got != "Не найдено" {
		t.Fatalf("родной формат: %q", got)
	}
}

func TestKeyAndWaitTask(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer vtk_test" {
			w.WriteHeader(401)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/vm/v3/task/7"):
			if atomic.AddInt32(&calls, 1) < 2 {
				_, _ = w.Write([]byte(`{"task":{"status":"running"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"task":{"status":"complete"}}`))
		case strings.HasPrefix(r.URL.Path, "/vm/v3/task/8"):
			_, _ = w.Write([]byte(`{"task":{"status":"failed","output":"нет места"}}`))
		case r.URL.Path == "/vm/v3/host/404":
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":404,"msg":"Хост не найден"}}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL+"/", "vtk_test")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.WaitTask(ctx, 7, time.Minute); err != nil {
		t.Fatalf("задача 7: %v", err)
	}
	if err := c.WaitTask(ctx, 8, time.Minute); err == nil || !strings.Contains(err.Error(), "нет места") {
		t.Fatalf("задача 8 должна упасть с причиной: %v", err)
	}
	if err := c.WaitTask(ctx, 0, time.Minute); err != nil {
		t.Fatalf("задача 0 — нечего ждать: %v", err)
	}
	if _, err := c.GetHost(ctx, 404); !IsNotFound(err) {
		t.Fatalf("404 должен распознаваться: %v", err)
	}
}
