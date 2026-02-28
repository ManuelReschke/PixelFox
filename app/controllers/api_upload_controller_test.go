package controllers

import (
	"testing"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/stretchr/testify/assert"
)

func TestResolvePublicUploadURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pool *models.StoragePool
		want string
	}{
		{
			name: "nil pool falls back to public path",
			pool: nil,
			want: "/api/v1/upload",
		},
		{
			name: "public base url is preferred",
			pool: &models.StoragePool{
				PublicBaseURL: "https://cdn.pixelfox.cc",
				UploadAPIURL:  "http://localhost:8082/api/internal/upload",
			},
			want: "https://cdn.pixelfox.cc/api/v1/upload",
		},
		{
			name: "internal absolute upload endpoint is rewritten",
			pool: &models.StoragePool{
				UploadAPIURL: "http://localhost:8082/api/internal/upload",
			},
			want: "http://localhost:8082/api/v1/upload",
		},
		{
			name: "internal relative upload endpoint is rewritten",
			pool: &models.StoragePool{
				UploadAPIURL: "/api/internal/upload",
			},
			want: "/api/v1/upload",
		},
		{
			name: "generic upload suffix is normalized",
			pool: &models.StoragePool{
				UploadAPIURL: "http://localhost:8082/upload",
			},
			want: "http://localhost:8082/api/v1/upload",
		},
		{
			name: "already public upload path is preserved",
			pool: &models.StoragePool{
				UploadAPIURL: "https://cdn.pixelfox.cc/api/v1/upload",
			},
			want: "https://cdn.pixelfox.cc/api/v1/upload",
		},
		{
			name: "empty upload url falls back to public path",
			pool: &models.StoragePool{},
			want: "/api/v1/upload",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolvePublicUploadURL(tc.pool)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "/api/internal/")
		})
	}
}
