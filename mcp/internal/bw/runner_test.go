package bw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := helperMode(); mode != "" {
		runHelper(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helperMode() string {
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--bw-runner-helper" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func runHelper(mode string) {
	switch mode {
	case "env":
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
			"args": os.Args[1:],
			"env":  os.Environ(),
		}); err != nil {
			os.Exit(2)
		}
	case "fail":
		_, _ = os.Stderr.WriteString("boom")
		os.Exit(3)
	case "sleep":
		time.Sleep(2 * time.Second)
	default:
		os.Exit(2)
	}
}

func TestExecRunnerUsesExplicitEnvAndNoInteraction(t *testing.T) {
	t.Setenv("SENTINEL", "from-parent")

	runner := ExecRunner{Binary: os.Args[0], Timeout: 5 * time.Second}
	res, err := runner.Run(context.Background(), []string{"--bw-runner-helper", "env", "status"}, []string{
		"ONLY=child",
	})
	if err != nil {
		t.Fatalf("run: %v\nstderr=%s", err, res.Stderr)
	}

	var got struct {
		Args []string `json:"args"`
		Env  []string `json:"env"`
	}
	if err := json.Unmarshal(res.Stdout, &got); err != nil {
		t.Fatalf("parse helper output: %v\n%s", err, res.Stdout)
	}
	wantArgs := []string{"--nointeraction", "--bw-runner-helper", "env", "status"}
	if strings.Join(got.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("args = %#v, want %#v", got.Args, wantArgs)
	}
	if strings.Join(got.Env, "\x00") != "ONLY=child" {
		t.Fatalf("env = %#v, want only the child environment", got.Env)
	}
}

func TestExecRunnerNilEnvDoesNotInherit(t *testing.T) {
	t.Setenv("SENTINEL", "from-parent")

	runner := ExecRunner{Binary: os.Args[0], Timeout: 5 * time.Second}
	res, err := runner.Run(context.Background(), []string{"--bw-runner-helper", "env"}, nil)
	if err != nil {
		t.Fatalf("run: %v\nstderr=%s", err, res.Stderr)
	}
	if bytes.Contains(res.Stdout, []byte("SENTINEL=")) {
		t.Fatalf("parent environment leaked: %s", res.Stdout)
	}
	var got struct {
		Env []string `json:"env"`
	}
	if err := json.Unmarshal(res.Stdout, &got); err != nil {
		t.Fatalf("parse helper output: %v\n%s", err, res.Stdout)
	}
	if len(got.Env) != 0 {
		t.Fatalf("env = %#v, want empty", got.Env)
	}
}

func TestExecRunnerExitCode(t *testing.T) {
	runner := ExecRunner{Binary: os.Args[0], Timeout: 5 * time.Second}
	res, err := runner.Run(context.Background(), []string{"--bw-runner-helper", "fail"}, nil)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v, want ExitError", err)
	}
	if exitErr.ExitCode != 3 || res.ExitCode != 3 {
		t.Fatalf("exit = %d result %d, want 3", exitErr.ExitCode, res.ExitCode)
	}
	if !bytes.Equal(res.Stderr, []byte("boom")) {
		t.Fatalf("stderr = %q", res.Stderr)
	}
	if bytes.Contains([]byte(err.Error()), []byte("boom")) {
		t.Fatalf("ExitError includes stderr: %v", err)
	}
}

func TestExecRunnerTimeout(t *testing.T) {
	runner := ExecRunner{Binary: os.Args[0], Timeout: 200 * time.Millisecond}
	_, err := runner.Run(context.Background(), []string{"--bw-runner-helper", "sleep"}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestExecRunnerRejectsSecretArgs(t *testing.T) {
	runner := ExecRunner{Binary: "/nonexistent/bw"}
	cases := [][]string{
		{"list", "items", "--session", "key"},
		{"list", "items", "--session=key"},
		{"unlock", "BW_PASSWORD=hunter2"},
		{"login", "BW_CLIENTSECRET=secret"},
		{"sync", "BW_SESSION=key"},
	}
	for _, args := range cases {
		_, err := runner.Run(context.Background(), args, nil)
		if err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Fatalf("args %q: error = %v, want refusal before exec", args, err)
		}
	}
}
