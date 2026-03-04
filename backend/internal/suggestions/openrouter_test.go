package suggestions

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsRetryableTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "deadline exceeded",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "wrapped deadline exceeded",
			err:  fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
			want: true,
		},
		{
			name: "net timeout",
			err:  timeoutErr{},
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("boom"),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isRetryableTimeout(tt.err); got != tt.want {
				t.Fatalf("isRetryableTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}
