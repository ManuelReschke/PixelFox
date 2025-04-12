package hcaptcha

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ManuelReschke/PixelFox/internal/pkg/env"
)

type Response struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

func Verify(token string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("hCaptcha token is empty")
	}

	secret := env.GetEnv("HCAPTCHA_SECRET", "")
	if secret == "" {
		return false, fmt.Errorf("hCaptcha secret is not set")
	}

	formData := url.Values{
		"secret":   {secret},
		"response": {token},
	}

	resp, err := http.PostForm("https://hcaptcha.com/siteverify", formData)
	if err != nil {
		return false, fmt.Errorf("failed to send request to hCaptcha API: %v", err)
	}
	defer resp.Body.Close()

	var response Response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return false, fmt.Errorf("failed to decode hCaptcha API response: %v", err)
	}

	if !response.Success {
		errorMsg := "hCaptcha validation failed"
		if len(response.ErrorCodes) > 0 {
			errorMsg = errorMsg + ": " + strings.Join(response.ErrorCodes, ", ")
		}
		return false, fmt.Errorf(errorMsg)
	}

	return true, nil
}
