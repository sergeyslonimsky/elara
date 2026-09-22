package cli

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRootCmd_ServeDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantServed bool
	}{
		{
			name:       "no args runs serve",
			args:       nil,
			wantServed: true,
		},
		{
			name:       "serve subcommand runs serve",
			args:       []string{"serve"},
			wantServed: true,
		},
		{
			name:       "version subcommand does not run serve",
			args:       []string{"version"},
			wantServed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			served := false
			serve := func() error {
				served = true

				return nil
			}

			root := newRootCmd(serve, VersionInfo{Version: "test"})
			root.SetArgs(tt.args)
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})

			require.NoError(t, root.Execute())
			require.Equal(t, tt.wantServed, served)
		})
	}
}

func TestNewRootCmd_ServeErrorPropagates(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	serve := func() error {
		return wantErr
	}

	root := newRootCmd(serve, VersionInfo{Version: "test"})
	root.SetArgs(nil)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	require.ErrorIs(t, err, wantErr)
}

func TestNewVersionCmd_PrintsBuildInfo(t *testing.T) {
	t.Parallel()

	info := VersionInfo{
		Version:   "v1.2.3",
		Commit:    "abc123",
		Date:      "2026-01-01",
		GoVersion: "go1.99",
		OS:        "linux",
		Arch:      "amd64",
	}

	root := newRootCmd(func() error { return nil }, info)
	out := &bytes.Buffer{}
	root.SetArgs([]string{"version"})
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})

	require.NoError(t, root.Execute())

	got := out.String()
	for _, want := range []string{
		"elara v1.2.3", "abc123", "2026-01-01", "go1.99", "linux/amd64",
	} {
		require.Contains(t, got, want)
	}
}

// os.Args, which cobra reads internally regardless of SetArgs on other
// commands — confirmed via -race that running this in parallel with the
// rest of the package causes a real data race.
//
//nolint:paralleltest // deliberately serial: mutates the process-wide
func TestExecute_ReadsOSArgs(t *testing.T) {
	origArgs := os.Args
	os.Args = []string{"elara", "version"}
	t.Cleanup(func() { os.Args = origArgs })

	err := Execute(func() error { return nil }, VersionInfo{Version: "test"})
	require.NoError(t, err)
}
