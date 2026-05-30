package proxtest

import (
	"context"
	"net/http"
	"testing"

	"github.com/anomalyco/proxmox-mcp/internal/proxmox"
	"go.uber.org/goleak"
	"gopkg.in/h2non/gock.v1"
)

type Route struct {
	Method string
	Path   string
	Status int
	Body   any
}

const (
	baseHost = "https://pve:8006"
	basePath = "/api2/json"
)

func New(t *testing.T, routes ...Route) *proxmox.Service {
	t.Helper()

	for _, r := range routes {
		m := gock.New(baseHost).Path(basePath + r.Path)
		m.Method = r.Method

		status := r.Status
		if status == 0 {
			status = 200
		}
		resp := m.Reply(status)

		if r.Body != nil {
			resp.JSON(r.Body)
		}
	}

	svc, err := proxmox.NewService(proxmox.Config{
		Host:        "pve",
		TokenID:     "test-token-id",
		TokenSecret: "test-token-secret",
	}, proxmox.WithHTTPClient(&http.Client{Transport: gock.DefaultTransport}))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if !gock.IsDone() {
			t.Error("unconsumed mock routes remain")
		}
		gock.OffAll()
		goleak.VerifyNone(t,
			goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
			goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		)
	})

	return svc
}

func NewCanceled(t *testing.T, routes ...Route) (*proxmox.Service, context.Context) {
	svc := New(t, routes...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return svc, ctx
}
