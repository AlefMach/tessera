package localexec

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/alef-mach/tessera/internal/port"
)

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) Run(ctx context.Context, command port.Command) (port.Output, error) {
	if command.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, command.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	cmd.Env = command.Env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := port.Output{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		output.ExitCode = exitErr.ExitCode()
	}
	return output, err
}
