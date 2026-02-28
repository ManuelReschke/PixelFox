package shortener

import (
	"strings"
	"testing"
)

func TestGenerateSecureSlug_InvalidLength(t *testing.T) {
	t.Parallel()

	if _, err := GenerateSecureSlug(0); err == nil {
		t.Fatalf("expected error for invalid length")
	}
}

func TestGenerateSecureSlug_LengthAndAlphabet(t *testing.T) {
	t.Parallel()

	slug, err := GenerateSecureSlug(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slug) != 10 {
		t.Fatalf("expected slug length 10, got %d", len(slug))
	}

	for i := 0; i < len(slug); i++ {
		if strings.IndexByte(alphabet, slug[i]) == -1 {
			t.Fatalf("slug contains invalid character %q", slug[i])
		}
	}
}

func TestGenerateSecureSlug_UniqueWithinSmallBatch(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})

	for i := 0; i < 100; i++ {
		slug, err := GenerateSecureSlug(10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := seen[slug]; exists {
			t.Fatalf("duplicate slug generated in small batch: %s", slug)
		}
		seen[slug] = struct{}{}
	}
}
