package workload

import (
	"bytes"
	"context"
	"os/exec"
)

type backgroundPriorityKey struct{}

func WithBackgroundPriority(ctx context.Context) context.Context {
	return context.WithValue(ctx, backgroundPriorityKey{}, true)
}

func BackgroundPriority(ctx context.Context) bool {
	marked, _ := ctx.Value(backgroundPriorityKey{}).(bool)
	return marked
}

func RunContext(ctx context.Context, command *exec.Cmd) error {
	if BackgroundPriority(ctx) {
		return RunBackground(command)
	}
	return command.Run()
}

func RunBackground(command *exec.Cmd) error {
	if err := startBackground(command); err != nil {
		return err
	}
	return command.Wait()
}

func BackgroundOutput(command *exec.Cmd) ([]byte, error) {
	var output bytes.Buffer
	command.Stdout = &output
	err := RunBackground(command)
	return output.Bytes(), err
}

func BackgroundCombinedOutput(command *exec.Cmd) ([]byte, error) {
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	err := RunBackground(command)
	return output.Bytes(), err
}
