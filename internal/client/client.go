// Package client — HTTP-клиент панели для провайдера Terraform vmpanel.
//
// Машины ведутся через совместимый API /vm/v3 (он одинаков для клиента
// хостера, его сотрудников и владельца платформы — права определяет ключ),
// вебхуки — через родной /api. Вход — ключом API (vtk_…) в Authorization.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	base string
	key  string
	http *http.Client
}

func New(endpoint, apiKey string) *Client {
	return &Client{
		base: strings.TrimRight(endpoint, "/"),
		key:  apiKey,
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

// APIError — ошибка панели. Оба формата: /vm/v3 {"error":{"code","msg"}},
// /api {"error":"код","message":"…"}.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("панель ответила %d: %s", e.Status, e.Message)
}

func IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return &APIError{Status: resp.StatusCode, Message: errorMessage(data)}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func errorMessage(data []byte) string {
	var compat struct {
		Error struct {
			Msg string `json:"msg"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &compat) == nil && compat.Error.Msg != "" {
		return compat.Error.Msg
	}
	var native struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &native) == nil && native.Message != "" {
		return native.Message
	}
	return strings.TrimSpace(string(data))
}

// ===== /vm/v3 =====

type Host struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Cluster struct {
		ID int64 `json:"id"`
	} `json:"cluster"`
	Node struct {
		Name string `json:"name"`
	} `json:"node"`
	CPU     int64 `json:"cpu_number"`
	RAMMiB  int64 `json:"ram_mib"`
	DiskMiB int64 `json:"total_disks_size_mib"`
	IP4     []struct {
		IP string `json:"ip"`
	} `json:"ip4"`
	OSName string `json:"os_name"`
}

type TaskRef struct {
	ID   int64 `json:"id"`
	Task int64 `json:"task"`
}

type Named struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	CPU    int64  `json:"cpu_number"`
	RAMMiB int64  `json:"ram_mib"`
	HddMiB int64  `json:"hdd_mib"`
}

func (c *Client) GetHost(ctx context.Context, id int64) (*Host, error) {
	var h Host
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/vm/v3/host/%d", id), nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// List — справочник /vm/v3 (os, cluster, preset) в виде {list: [...]}.
func (c *Client) List(ctx context.Context, what string) ([]Named, error) {
	var out struct {
		List []Named `json:"list"`
	}
	if err := c.Do(ctx, http.MethodGet, "/vm/v3/"+what, nil, &out); err != nil {
		return nil, err
	}
	return out.List, nil
}

// WaitTask ждёт задачу до complete; failed — ошибка с причиной. 0 — задачи нет.
func (c *Client) WaitTask(ctx context.Context, id int64, timeout time.Duration) error {
	if id == 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for {
		var out struct {
			Task struct {
				Status string  `json:"status"`
				Output *string `json:"output"`
			} `json:"task"`
		}
		if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/vm/v3/task/%d", id), nil, &out); err != nil {
			return err
		}
		switch out.Task.Status {
		case "complete":
			return nil
		case "failed":
			msg := "без подробностей"
			if out.Task.Output != nil && *out.Task.Output != "" {
				msg = *out.Task.Output
			}
			return fmt.Errorf("задача %d не выполнена: %s", id, msg)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("задача %d не завершилась за %s", id, timeout)
		}
		if err := sleep(ctx, 5*time.Second); err != nil {
			return err
		}
	}
}

// WaitActive ждёт, пока машина станет active (адрес и пароль уже в госте).
func (c *Client) WaitActive(ctx context.Context, id int64, timeout time.Duration) (*Host, error) {
	deadline := time.Now().Add(timeout)
	for {
		h, err := c.GetHost(ctx, id)
		if err != nil {
			return nil, err
		}
		switch h.State {
		case "active":
			return h, nil
		case "crashed":
			return h, fmt.Errorf("сборка машины %d сорвалась", id)
		}
		if time.Now().After(deadline) {
			return h, fmt.Errorf("машина %d не стала active за %s (сейчас %s)", id, timeout, h.State)
		}
		if err := sleep(ctx, 5*time.Second); err != nil {
			return nil, err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
