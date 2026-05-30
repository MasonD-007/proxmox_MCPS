package proxmox

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	goproxmox "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
)

func TestConfigLoad(t *testing.T) {
	t.Setenv("PROXMOX_HOST", "pve1")
	t.Setenv("PROXMOX_TOKEN_ID", "root@pam!test")
	t.Setenv("PROXMOX_TOKEN_SECRET", "secret123")
	t.Setenv("PROXMOX_INSECURE_TLS", "true")

	var cfg Config
	err := cfg.Load()
	require.NoError(t, err)
	assert.Equal(t, "pve1", cfg.Host)
	assert.Equal(t, "root@pam!test", cfg.TokenID)
	assert.Equal(t, "secret123", cfg.TokenSecret)
	assert.True(t, cfg.InsecureTLS)
	assert.Equal(t, "https://pve1:8006/api2/json", cfg.BaseURL())
}

func TestConfigLoadMissingRequired(t *testing.T) {
	os.Unsetenv("PROXMOX_HOST")
	os.Unsetenv("PROXMOX_TOKEN_ID")
	os.Unsetenv("PROXMOX_TOKEN_SECRET")

	var cfg Config
	err := cfg.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PROXMOX_HOST")
}

func TestNewService(t *testing.T) {
	svc, err := NewService(Config{
		Host:        "pve",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	}, WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	require.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestVersionSmoke(t *testing.T) {
	defer gock.Off()

	m := gock.New("https://pve:8006").Path("/api2/json/version")
	m.Method = "GET"
	m.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"release": "8.1",
			"repoid":  "abc123",
			"version": "8.1-1",
		},
	})

	svc, err := NewService(Config{
		Host:        "pve",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	}, WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	require.NoError(t, err)

	v, err := svc.Client().Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8.1", v.Release)
	assert.Equal(t, "8.1-1", v.Version)
}

func TestNodeLookup(t *testing.T) {
	defer gock.Off()

	m := gock.New("https://pve:8006").Path("/api2/json/nodes/pve/status")
	m.Method = "GET"
	m.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"cpu":     0.05,
			"maxcpu":  8,
			"maxmem":  34359738368,
			"mem":     8589934592,
			"node":    "pve",
			"status":  "online",
			"uptime":  123456,
		},
	})

	svc, err := NewService(Config{
		Host:        "pve",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	}, WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	require.NoError(t, err)

	n, err := svc.node(context.Background(), "pve")
	require.NoError(t, err)
	assert.Equal(t, "pve", n.Name)
	assert.InDelta(t, 0.05, n.CPU, 0.001)
	assert.Equal(t, uint64(123456), n.Uptime)
}

func TestVMLookup(t *testing.T) {
	defer gock.Off()

	m1 := gock.New("https://pve:8006").Path("/api2/json/nodes/pve/status")
	m1.Method = "GET"
	m1.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"node":   "pve",
			"status": "online",
		},
	})

	m2 := gock.New("https://pve:8006").Path("/api2/json/nodes/pve/qemu/100/status/current")
	m2.Method = "GET"
	m2.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"vmid":   100,
			"name":   "test-vm",
			"status": "running",
		},
	})

	m3 := gock.New("https://pve:8006").Path("/api2/json/nodes/pve/qemu/100/config")
	m3.Method = "GET"
	m3.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"name":  "test-vm",
			"cores": 2,
		},
	})

	svc, err := NewService(Config{
		Host:        "pve",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	}, WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	require.NoError(t, err)

	vm, err := svc.vm(context.Background(), "pve", 100)
	require.NoError(t, err)
	assert.Equal(t, "test-vm", vm.Name)
	assert.Equal(t, "running", vm.Status)
}

func TestContainerLookup(t *testing.T) {
	defer gock.Off()

	m1 := gock.New("https://pve:8006").Path("/api2/json/nodes/pve/status")
	m1.Method = "GET"
	m1.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"node":   "pve",
			"status": "online",
		},
	})

	m2 := gock.New("https://pve:8006").Path("/api2/json/nodes/pve/lxc/200/status/current")
	m2.Method = "GET"
	m2.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"vmid":   200,
			"name":   "test-ct",
			"status": "running",
		},
	})

	m3 := gock.New("https://pve:8006").Path("/api2/json/nodes/pve/lxc/200/config")
	m3.Method = "GET"
	m3.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"hostname": "test-ct",
		},
	})

	svc, err := NewService(Config{
		Host:        "pve",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	}, WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	require.NoError(t, err)

	ct, err := svc.container(context.Background(), "pve", 200)
	require.NoError(t, err)
	assert.Equal(t, "test-ct", ct.Name)
	assert.Equal(t, "running", ct.Status)
}

func TestWaitTaskCompletes(t *testing.T) {
	defer gock.Off()

	path := "/api2/json/nodes/node1/tasks/UPID:node1:00000001:00000001:00000001:test:completed:root@pam:/status"
	m := gock.New("https://pve:8006").Path(path)
	m.Method = "GET"
	m.Times(2)
	m.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"status":     "stopped",
			"exitstatus": "OK",
			"upid":       "UPID:node1:00000001:00000001:00000001:test:completed:root@pam:",
			"node":       "node1",
		},
	})

	svc, err := NewService(Config{
		Host:        "pve",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	}, WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	require.NoError(t, err)

	task := goproxmox.NewTask(
		goproxmox.UPID("UPID:node1:00000001:00000001:00000001:test:completed:root@pam:"),
		svc.Client(),
	)
	require.NotNil(t, task)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = svc.wait(ctx, task)
	assert.NoError(t, err)
}

func TestWaitContextCanceled(t *testing.T) {
	defer gock.Off()

	path := "/api2/json/nodes/node1/tasks/UPID:node2:00000002:00000002:00000002:test:running:root@pam:/status"
	m := gock.New("https://pve:8006").Path(path)
	m.Method = "GET"
	m.Times(2)
	m.Reply(200).JSON(map[string]any{
		"data": map[string]any{
			"status": "running",
			"upid":   "UPID:node2:00000002:00000002:00000002:test:running:root@pam:",
			"node":   "node1",
		},
	})

	svc, err := NewService(Config{
		Host:        "pve",
		TokenID:     "dummy",
		TokenSecret: "dummy",
	}, WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	require.NoError(t, err)

	task := goproxmox.NewTask(
		goproxmox.UPID("UPID:node2:00000002:00000002:00000002:test:running:root@pam:"),
		svc.Client(),
	)
	require.NotNil(t, task)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = svc.wait(ctx, task)
	assert.Error(t, err)
}
