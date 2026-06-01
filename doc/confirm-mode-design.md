# Confirm Mode Design

## Current Behavior

Before this change, `DROP TABLE` and `TRUNCATE TABLE` were absolutely blocked by the `guard.PolicyBlock` default. The only way to execute them was to explicitly configure `guard.PolicyWarn` or `guard.PolicyAllow` at the library level.

## Target Behavior

When a DROP or TRUNCATE is attempted, the user is prompted for confirmation. If confirmed (or if `--force` is passed), the operation proceeds. If denied, the operation is rejected with an error.

## Design

### 1. Guard Package (`pkg/guard`)

Add a new policy mode: `PolicyPrompt`

```go
const (
    PolicyBlock  Policy = iota  // Block with ErrDangerousOp
    PolicyWarn                  // Allow, log warning
    PolicyAllow                 // Allow silently
    PolicyPrompt                // Return confirmable error
)
```

Add a new sentinel error: `ErrDangerousOpPrompt`

```go
var ErrDangerousOpPrompt = errors.New("dangerous operation requires confirmation")
```

`Check()` returns `ErrDangerousOpPrompt` when:
- Policy is `PolicyPrompt`
- The SQL is a dangerous operation (DROP/TRUNCATE)

```go
func Check(policy Policy, sql string) error {
    if policy == PolicyAllow {
        return nil
    }
    if !IsDangerousOp(sql) {
        return nil
    }
    switch policy {
    case PolicyBlock:
        return ErrDangerousOp
    case PolicyPrompt:
        return ErrDangerousOpPrompt
    default:
        return nil // PolicyWarn
    }
}
```

The error is wrappable and reflectable via `errors.Is()`:

```go
if errors.Is(err, guard.ErrDangerousOpPrompt) {
    // prompt user
}
```

### 2. Config Package (`pkg/config`)

Change the default policy from `PolicyBlock` to `PolicyPrompt`:

```go
func DefaultConfig() *Config {
    return &Config{
        ...
        DangerousOpPolicy: guard.PolicyPrompt,  // was guard.PolicyBlock
        ...
    }
}
```

All other policy options remain unchanged. Users who want the old strict behavior can explicitly configure `WithDangerousOpPolicy(guard.PolicyBlock)`.

### 3. CLI (`cmd/cli/main.go`)

#### `--force` Flag

```go
var force bool

func init() {
    flag.BoolVar(&force, "force", false, "skip confirmation prompts for dangerous operations")
}
```

#### Inline exec flow

```go
func execWithConfirmation(ctx context.Context, sess *db.Session, sql string) (*result.ExecResult, error) {
    res, err := sess.Exec(ctx, sql)
    if err == nil || !errors.Is(err, guard.ErrDangerousOpPrompt) {
        return res, err
    }

    // If --force is set, auto-confirm
    if force {
        sess.Config().DangerousOpPolicy = guard.PolicyAllow
        return sess.Exec(ctx, sql)
    }

    // Interactive prompt
    fmt.Fprintf(os.Stderr, "WARNING: %q\nType 'yes' to confirm: ", sql)
    reader := bufio.NewReader(os.Stdin)
    response, _ := reader.ReadString('\n')
    response = strings.TrimSpace(strings.ToLower(response))

    if response == "yes" {
        sess.Config().DangerousOpPolicy = guard.PolicyAllow
        return sess.Exec(ctx, sql)
    }

    return nil, fmt.Errorf("dangerous operation cancelled by user: %s", sql)
}
```

#### Batch exec flow

For batch mode (`exec --file`), the `--force` flag sets `guard.PolicyAllow` before batch execution starts. Without `--force`, dangerous operations in batch files fail with the prompt error (the file-based flow is non-interactive by design).

```go
if filePath != "" {
    ...
    if force {
        sess.Config().DangerousOpPolicy = guard.PolicyAllow
    }
    res := batchExec(ctx, sess, statements, ...)
}
```

### 4. Executor (`pkg/db/executor.go`)

No changes needed. The executor calls `guard.Check()` which now returns `ErrDangerousOpPrompt` under `PolicyPrompt`. The error is wrapped with context:

```go
if err := guard.Check(s.cfg.DangerousOpPolicy, sqlStr); err != nil {
    return nil, fmt.Errorf("exec %s: %w", op, err)
}
```

The wrapper preserves `errors.Is()` behavior because Go's `%w` verb preserves the error chain.

### 5. Transaction Executor (`pkg/db/transaction.go`)

Same as the session executor — no changes needed. The same `guard.Check()` call is used in `transaction.Exec()`.

### 6. Testing Impact

| Test | Status | Notes |
|------|--------|-------|
| `TestExecDropTableBlocked` | Passes | Still expects any error |
| `TestExecTruncateBlocked` | Passes | Still expects any error |
| `TestExecDropWithPolicyWarn` | Passes | Explicitly sets PolicyWarn |
| `TestTxExecBlocksDrop` | Passes | Still expects any error |
| `TestTxWithPolicyWarnAllowsDrop` | Passes | Explicitly sets PolicyWarn |
| `TestDefaultConfig` | Updated | Expects PolicyPrompt not PolicyBlock |

No test needs to change behavior — they either check for any error (which PolicyPrompt still produces) or explicitly set their own policy.

## Error Message Flow

```
CLI user runs: qc exec "root@tcp(...)/db" "DROP TABLE users"

1. Session.Exec() → guard.Check(PolicyPrompt, "DROP TABLE users") → ErrDangerousOpPrompt
2. Error propagates to CLI as: "exec DROP: dangerous operation requires confirmation"
3. execWithConfirmation catches errors.Is(ErrDangerousOpPrompt)
4. Prints: WARNING: "DROP TABLE users"
           Type 'yes' to confirm:
5. User types: yes
6. Session config set to PolicyAllow
7. Session.Exec() retried → succeeds
8. Result printed as JSON
```

## Config Persistence Note

The session's `Config.DangerousOpPolicy` is a pointer field. Setting it to `PolicyAllow` for confirmation changes the session permanently for the lifetime of the CLI process. This is acceptable because:

- In the CLI, each invocation creates a fresh session
- In library usage, the caller manages config explicitly
- A single process rarely needs to toggle confirmation per-statement

If per-statement toggling were needed, the guard.Check call could accept a one-time override parameter. This is not needed for v1.

## Backward Compatibility

- Code using `PolicyBlock` explicitly: unaffected (still works the same)
- Code using default config: behavior changes from block to prompt
- CLI users: DROP/TRUNCATE now prompt instead of failing with an error
- Library users calling `sess.Exec()`: receive `ErrDangerousOpPrompt` instead of `ErrDangerousOp` from the default config — check with `errors.Is()` to handle either
