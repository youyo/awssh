package awssh

import (
	"context"
	"os"
	"os/exec"
	"time"
)

const (
	CmdSessionManagerPlugin      string = "session-manager-plugin"
	CmdSessionManagerPluginOrder string = "StartSession"
)

func execExternalCommand(ctx context.Context, externalCommand string, args []string) (command *exec.Cmd, err error) {
	command = exec.CommandContext(ctx, externalCommand, args[0:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	err = command.Start()
	return command, err
}

func execSessionManagerPortForwarding(ctx context.Context, tokens, region, sessionManagerParam, url string) (command *exec.Cmd, err error) {
	args := []string{tokens, region, CmdSessionManagerPluginOrder, "", sessionManagerParam, url}
	command, err = execExternalCommand(ctx, CmdSessionManagerPlugin, args)
	time.Sleep(1 * time.Second)
	return command, err
}
