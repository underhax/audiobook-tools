package cli

import (
	"bytes"
	"testing"
)

type failWriter struct{}

func (w failWriter) Write(_ []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}

type failReader struct{}

func (r failReader) Read(_ []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}

func TestAskRetry(t *testing.T) {
	tests := []struct {
		in       *bytes.Buffer
		out      *bytes.Buffer
		name     string
		badWrite bool
		badRead  bool
		want     bool
	}{
		{
			name: "yes",
			in:   bytes.NewBufferString("yes\n"),
			out:  &bytes.Buffer{},
			want: true,
		},
		{
			name: "no",
			in:   bytes.NewBufferString("no\n"),
			out:  &bytes.Buffer{},
			want: false,
		},
		{
			name:     "write err",
			badWrite: true,
			in:       bytes.NewBufferString("yes\n"),
			want:     false,
		},
		{
			name:    "read err",
			badRead: true,
			out:     &bytes.Buffer{},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w interface{ Write([]byte) (int, error) } = tt.out
			if tt.badWrite {
				w = failWriter{}
			}
			var r interface{ Read([]byte) (int, error) } = tt.in
			if tt.badRead {
				r = failReader{}
			}

			if got := askRetry(r, w); got != tt.want {
				t.Errorf("askRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
