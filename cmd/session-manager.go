package cmd

import (
	"context"
	"os/exec"
	"time"
)

const (
	CmdSessionManagerPlugin      string = "session-manager-plugin"
	CmdSessionManagerPluginOrder string = "StartSession"
)

func ExecSessionManagerPortForwarding(ctx context.Context, tokens, region, sessionManagerParam, url string) (command *exec.Cmd, err error) {
	args := []string{tokens, region, CmdSessionManagerPluginOrder, "", sessionManagerParam, url}
	command, err = ExecExternalCommand(ctx, CmdSessionManagerPlugin, args)
	time.Sleep(1 * time.Second)
	return command, err
}
