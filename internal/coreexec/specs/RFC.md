# coreexec
**Import:** `forge.lthn.ai/core/go-infra/internal/coreexec`
**Files:** 1

## Types

### `Result`
Captured output and exit status returned by `Run`.
- `Stdout string`: Standard output collected from the child process.
- `Stderr string`: Standard error collected from the child process.
- `ExitCode int`: Exit code derived from the child process wait status. Signalled processes are reported as `128 + signal`.

## Functions

### `func LookPath(name string) (string, error)`
Resolves an executable name against `PATH`, accepting both absolute paths and relative path-like inputs, and verifies execute permission before returning the resolved path.

### `func Run(ctx context.Context, name string, args ...string) (Result, error)`
Forks and executes a command, captures `stdout` and `stderr` to temporary files, waits for completion or context cancellation, and returns the resulting `Result`.

### `func Exec(name string, args ...string) error`
Replaces the current process image with the named executable using `syscall.Exec`.
