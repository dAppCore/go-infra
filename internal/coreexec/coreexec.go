package coreexec

import (
	"context"
	"syscall"

	core "dappco.re/go/core"
)

var localFS = (&core.Fs{}).NewUnrestricted()

const executeAccess = 1

// Result captures process output and exit status.
// Usage: result, err := coreexec.Run(ctx, "git", "status", "--short")
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// LookPath resolves an executable name against PATH.
// Usage: path, err := coreexec.LookPath("gh")
func LookPath(name string) (string, error) {
	if name == "" {
		return "", core.E("coreexec.LookPath", "empty executable name", nil)
	}

	ds := core.Env("DS")
	if core.PathIsAbs(name) || core.Contains(name, ds) {
		if isExecutable(name) {
			return name, nil
		}
		return "", core.E("coreexec.LookPath", core.Concat("executable not found: ", name), nil)
	}

	for _, dir := range core.Split(core.Env("PATH"), core.Env("PS")) {
		if dir == "" {
			dir = core.Env("DIR_CWD")
		} else if !core.PathIsAbs(dir) {
			dir = core.Path(core.Env("DIR_CWD"), dir)
		}
		candidate := core.Path(dir, name)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	return "", core.E("coreexec.LookPath", core.Concat("executable not found in PATH: ", name), nil)
}

// Run executes a command and captures stdout, stderr, and exit status.
// Usage: result, err := coreexec.Run(ctx, "gh", "api", "repos/org/repo")
func Run(ctx context.Context, name string, args ...string) (Result, error) {
	path, err := LookPath(name)
	if err != nil {
		return Result{}, err
	}

	tempDir := localFS.TempDir("coreexec-")
	if tempDir == "" {
		return Result{}, core.E("coreexec.Run", "create capture directory", nil)
	}
	defer func() { _ = coreResultErr(localFS.DeleteAll(tempDir), "coreexec.Run") }()

	stdoutPath := core.Path(tempDir, "stdout")
	stderrPath := core.Path(tempDir, "stderr")

	stdoutFile, err := createFile(stdoutPath)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = stdoutFile.Close() }()

	stderrFile, err := createFile(stderrPath)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = stderrFile.Close() }()

	pid, err := syscall.ForkExec(path, append([]string{name}, args...), &syscall.ProcAttr{
		Dir: core.Env("DIR_CWD"),
		Env: syscall.Environ(),
		Files: []uintptr{
			0,
			stdoutFile.Fd(),
			stderrFile.Fd(),
		},
	})
	if err != nil {
		return Result{}, core.E("coreexec.Run", core.Concat("start ", name), err)
	}

	status, err := waitForPID(ctx, pid, name)
	if err != nil {
		return Result{}, err
	}

	stdout, err := readFile(stdoutPath)
	if err != nil {
		return Result{}, err
	}

	stderr, err := readFile(stderrPath)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode(status),
	}, nil
}

// Exec replaces the current process with the named executable.
// Usage: return coreexec.Exec("ssh", "-i", keyPath, host)
func Exec(name string, args ...string) error {
	path, err := LookPath(name)
	if err != nil {
		return err
	}

	if err := syscall.Exec(path, append([]string{name}, args...), syscall.Environ()); err != nil {
		return core.E("coreexec.Exec", core.Concat("exec ", name), err)
	}
	return nil
}

type captureFile interface {
	Close() error
	Fd() uintptr
}

type waitResult struct {
	status syscall.WaitStatus
	err    error
}

func isExecutable(path string) bool {
	if !localFS.IsFile(path) {
		return false
	}
	return syscall.Access(path, executeAccess) == nil
}

func createFile(path string) (captureFile, error) {
	created := localFS.Create(path)
	if !created.OK {
		return nil, core.E("coreexec.Run", core.Concat("create ", path), coreResultErr(created, "coreexec.Run"))
	}

	file, ok := created.Value.(captureFile)
	if !ok {
		return nil, core.E("coreexec.Run", core.Concat("capture handle type for ", path), nil)
	}
	return file, nil
}

func readFile(path string) (string, error) {
	read := localFS.Read(path)
	if !read.OK {
		return "", core.E("coreexec.Run", core.Concat("read ", path), coreResultErr(read, "coreexec.Run"))
	}

	content, ok := read.Value.(string)
	if !ok {
		return "", core.E("coreexec.Run", core.Concat("unexpected content type for ", path), nil)
	}
	return content, nil
}

func waitForPID(ctx context.Context, pid int, name string) (syscall.WaitStatus, error) {
	done := make(chan waitResult, 1)
	go func() {
		var status syscall.WaitStatus
		_, err := syscall.Wait4(pid, &status, 0, nil)
		done <- waitResult{status: status, err: err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			return 0, core.E("coreexec.Run", core.Concat("wait ", name), result.err)
		}
		return result.status, nil
	case <-ctx.Done():
		_ = syscall.Kill(pid, syscall.SIGKILL)
		result := <-done
		if result.err != nil {
			return 0, core.E("coreexec.Run", core.Concat("wait ", name), result.err)
		}
		return 0, core.E("coreexec.Run", core.Concat("command cancelled: ", name), ctx.Err())
	}
}

func exitCode(status syscall.WaitStatus) int {
	if status.Exited() {
		return status.ExitStatus()
	}
	if status.Signaled() {
		return 128 + int(status.Signal())
	}
	return 1
}

func coreResultErr(r core.Result, op string) error {
	if r.OK {
		return nil
	}
	if err, ok := r.Value.(error); ok && err != nil {
		return err
	}
	if r.Value == nil {
		return core.E(op, "unexpected empty core result", nil)
	}
	return core.E(op, core.Sprint(r.Value), nil)
}
