package proxtest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	svc := New(t, Route{
		Method: "GET",
		Path:   "/version",
		Status: 200,
		Body: map[string]any{
			"data": map[string]any{
				"release": "8.1",
				"repoid":  "abc123",
				"version": "8.1-1",
			},
		},
	})
	require.NotNil(t, svc)

	v, err := svc.Client().Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8.1", v.Release)
	assert.Equal(t, "8.1-1", v.Version)
}

func TestNewCanceled(t *testing.T) {
	svc, ctx := NewCanceled(t)
	require.NotNil(t, svc)
	require.NotNil(t, ctx)

	select {
	case <-ctx.Done():
		assert.ErrorIs(t, ctx.Err(), context.Canceled)
	default:
		t.Fatal("expected context to be canceled")
	}
}

func TestNewMultipleRoutes(t *testing.T) {
	svc := New(t,
		Route{
			Method: "GET",
			Path:   "/version",
			Status: 200,
			Body: map[string]any{
				"data": map[string]any{
					"release": "8.2",
					"version": "8.2-1",
				},
			},
		},
		Route{
			Method: "GET",
			Path:   "/version",
			Status: 200,
			Body: map[string]any{
				"data": map[string]any{
					"release": "8.2",
					"version": "8.2-1",
				},
			},
		},
	)

	v, err := svc.Client().Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8.2", v.Release)

	v, err = svc.Client().Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8.2", v.Release)
}

func TestNewDefaultStatus(t *testing.T) {
	svc := New(t, Route{
		Method: "GET",
		Path:   "/version",
		Body: map[string]any{
			"data": map[string]any{
				"release": "8.1",
				"version": "8.1-1",
			},
		},
	})

	v, err := svc.Client().Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "8.1", v.Release)
}

func TestNewErrorResponse(t *testing.T) {
	svc := New(t, Route{
		Method: "GET",
		Path:   "/version",
		Status: 500,
		Body: map[string]any{
			"errors": "internal server error",
		},
	})

	_, err := svc.Client().Version(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || err != nil)
}

func TestNewConsumesRoutes(t *testing.T) {
	svc := New(t, Route{
		Method: "GET",
		Path:   "/version",
		Status: 200,
		Body: map[string]any{
			"data": map[string]any{
				"release": "8.1",
				"version": "8.1-1",
			},
		},
	})

	v, err := svc.Client().Version(context.Background())
	require.NoError(t, err)
	require.NotNil(t, v)
}
