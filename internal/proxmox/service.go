package proxmox

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	goproxmox "github.com/luthermonson/go-proxmox"
)

type Service struct {
	client *goproxmox.Client
}

type serviceConfig struct {
	httpClient *http.Client
}

type ServiceOption func(*serviceConfig)

func WithHTTPClient(client *http.Client) ServiceOption {
	return func(c *serviceConfig) {
		c.httpClient = client
	}
}

func NewService(cfg Config, opts ...ServiceOption) (*Service, error) {
	var sc serviceConfig
	for _, o := range opts {
		o(&sc)
	}

	baseURL := cfg.BaseURL()
	var clientOpts []goproxmox.Option

	if sc.httpClient != nil {
		clientOpts = append(clientOpts, goproxmox.WithHTTPClient(sc.httpClient))
	} else if cfg.InsecureTLS {
		clientOpts = append(clientOpts, goproxmox.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}))
	}

	clientOpts = append(clientOpts, goproxmox.WithAPIToken(cfg.TokenID, cfg.TokenSecret))
	client := goproxmox.NewClient(baseURL, clientOpts...)

	return &Service{client: client}, nil
}

func (s *Service) Client() *goproxmox.Client {
	return s.client
}

func (s *Service) node(ctx context.Context, name string) (*goproxmox.Node, error) {
	return s.client.Node(ctx, name)
}

func (s *Service) vm(ctx context.Context, node string, vmid uint64) (*goproxmox.VirtualMachine, error) {
	n, err := s.client.Node(ctx, node)
	if err != nil {
		return nil, err
	}
	return n.VirtualMachine(ctx, int(vmid))
}

func (s *Service) container(ctx context.Context, node string, vmid uint64) (*goproxmox.Container, error) {
	n, err := s.client.Node(ctx, node)
	if err != nil {
		return nil, err
	}
	return n.Container(ctx, int(vmid))
}

func (s *Service) wait(ctx context.Context, task *goproxmox.Task) error {
	return task.Wait(ctx, 1*time.Second, 5*time.Minute)
}
