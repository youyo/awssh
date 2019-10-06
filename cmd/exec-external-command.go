package cmd

import (
	"context"
	"os"
	"os/exec"
)

func ExecExternalCommand(ctx context.Context, externalCommand string, args []string) (command *exec.Cmd, err error) {
	command = exec.CommandContext(ctx, externalCommand, args[0:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	err = command.Start()
	return command, err
}
