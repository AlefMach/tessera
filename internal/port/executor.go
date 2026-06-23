package port

import (
	"context"
	"time"
)

type Command struct {
	Name    string
	Args    []string
	Dir     string
	Env     []string
	Timeout time.Duration
}

type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ToolExecutor interface {
	Run(ctx context.Context, cmd Command) (Output, error)
}
