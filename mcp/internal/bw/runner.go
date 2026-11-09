// Package bw runs the Bitwarden CLI and returns the vault model.
// Secrets are placed in the child environment, never in argv, and the server environment is not inherited.
// The session token stays inside CLI: Open creates it, Close locks and removes it.
package bw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const DefaultTimeout = 2 * time.Minute

// Result is the output of one bw invocation.
// Stdout may contain secrets. Callers must not log it.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Runner execs one bw invocation.
// args must not carry a master password or session key.
// env is the complete child environment; it is not merged with the server process environment.
type Runner interface {
	Run(ctx context.Context, args []string, env []string) (Result, error)
}

// ExitError is returned when bw exits non-zero.
// It does not include stdout or stderr; those stay on Result so callers can avoid logging secrets.
type ExitError struct {
	ExitCode int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.ExitCode)
}

// ExecRunner runs the bw binary.
// A nil environment is not inherited from the server process.
type ExecRunner struct {
	// Binary is the bw executable. Empty means "bw".
	Binary string
	// Timeout bounds each invocation. Empty means DefaultTimeout.
	// A shorter deadline already on ctx still wins.
	Timeout time.Duration
}

// Run execs bw with --nointeraction and an explicit child environment.
func (r ExecRunner) Run(ctx context.Context, args []string, env []string) (Result, error) {
	if err := validateArgs(args); err != nil {
		return Result{}, err
	}
	if env == nil {
		env = []string{}
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.binary(), append([]string{"--nointeraction"}, args...)...)
	cmd.Env = env
	cmd.Stdin = nil

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	if err == nil {
		return res, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, &ExitError{ExitCode: res.ExitCode}
	}
	return res, err
}

func (r ExecRunner) binary() string {
	if r.Binary != "" {
		return r.Binary
	}
	return "bw"
}

func validateArgs(args []string) error {
	for _, arg := range args {
		switch {
		case arg == "--session" || strings.HasPrefix(arg, "--session="):
			return errors.New("bw: refusing to pass --session as an argument")
		case strings.Contains(arg, "BW_SESSION=") || strings.Contains(arg, "BW_PASSWORD=") || strings.Contains(arg, "BW_CLIENTSECRET="):
			return errors.New("bw: refusing to pass secret material as an argument")
		}
	}
	return nil
}
