package controllers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ManuelReschke/PixelFox/internal/pkg/entitlements"
	"github.com/ManuelReschke/PixelFox/internal/pkg/usercontext"
)

func TestCalculateFileHash(t *testing.T) {
	const input = "pixelfox-upload"

	got, err := calculateFileHash(strings.NewReader(input))
	require.NoError(t, err)

	sum := sha256.Sum256([]byte(input))
	want := hex.EncodeToString(sum[:])
	assert.Equal(t, want, got)
}

func TestUploadWorkflowParseUploadForm_Success(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c *fiber.Ctx) error {
		w := &uploadWorkflow{c: c}
		form, file, err := w.parseUploadForm()
		require.NoError(t, err)
		require.NotNil(t, form)
		require.NotNil(t, file)
		assert.Equal(t, "test.png", file.Filename)
		_ = form.RemoveAll()
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := newMultipartUploadRequest(t, true)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
}

func TestUploadWorkflowParseUploadForm_MissingFile(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c *fiber.Ctx) error {
		w := &uploadWorkflow{c: c}
		_, _, err := w.parseUploadForm()
		require.Error(t, err)
		assert.True(t, errors.Is(err, errUploadResponseHandled))
		return nil
	})

	req := newMultipartUploadRequest(t, false)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusFound, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
}

func TestUploadWorkflowValidateEntitlements_FileTooLarge(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c *fiber.Ctx) error {
		w := &uploadWorkflow{
			c:       c,
			userCtx: usercontext.UserContext{UserID: 42, Plan: string(entitlements.PlanFree)},
		}

		file := &multipart.FileHeader{
			Filename: "too-large.png",
			Size:     entitlements.MaxUploadBytes(entitlements.PlanFree) + 1,
		}
		err := w.validateEntitlements(file)
		require.Error(t, err)
		assert.True(t, errors.Is(err, errUploadResponseHandled))
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode)
	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "Die Datei ist zu groß")
}

func TestUploadWorkflowRespondSuccessHTMX(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c *fiber.Ctx) error {
		w := &uploadWorkflow{c: c}
		return w.respondSuccess("test.png", "uuid-123")
	})

	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "/image/uuid-123", resp.Header.Get("HX-Redirect"))
	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "Datei erfolgreich hochgeladen")
}

func TestUploadWorkflowRespondSuccessRedirect(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c *fiber.Ctx) error {
		w := &uploadWorkflow{c: c}
		return w.respondSuccess("test.png", "uuid-123")
	})

	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusFound, resp.StatusCode)
	assert.Equal(t, "/image/uuid-123", resp.Header.Get("Location"))
}

func TestUploadWorkflowRunUnauthorized(t *testing.T) {
	app := fiber.New()
	app.Post("/upload", func(c *fiber.Ctx) error {
		w := &uploadWorkflow{
			c:       c,
			userCtx: usercontext.UserContext{IsLoggedIn: false},
		}
		return w.run()
	})

	req := httptest.NewRequest(http.MethodPost, "/upload", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	assert.Equal(t, "Unauthorized", string(body))
}

func newMultipartUploadRequest(t *testing.T, withFile bool) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if withFile {
		part, err := writer.CreateFormFile("file", "test.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("fake-image-content"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
