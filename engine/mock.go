package engine

import (
	"context"
	"strings"
)

// MockCaller is an offline Caller for smoke tests and --mock runs. It returns
// deterministic, valid JSON for both the persona fan-out and the synthesizer,
// so the full pipeline can be exercised without an API key or network.
type MockCaller struct{}

func (MockCaller) Chat(ctx context.Context, system, user string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		if strings.Contains(user, "\nPerspectives:\n") {
			return `{"feedback":"Mock review: promising but requires validation.","score":72}`, nil
		}
		return `{"summary":"Mock persona perspective.","key_points":["point one","point two"],"risks":["a risk"],"dependencies":["a dependency"],"score":60}`, nil
	}
}
