package cmd

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	InstanceID      string
	Username        string
	IdentityFile    string
	PublicKey       string
	Port            string
	ExternalCommand string
	Version         string
)

func init() {}

func NewCmdRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "awssh [instance-id]",
		Short:   "",
		Version: Version,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return errors.New("accepts only 1 arg")
			}

			if len(args) == 1 {
				instanceIdRe := regexp.MustCompile(`^i-([a-zA-Z0-9A0-zZ9]{8}|[a-zA-Z0-9A0-zZ9]{17})$`)
				if !instanceIdRe.MatchString(args[0]) {
					return errors.New("unmatched instance-id")
				}
			}

			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if PublicKey == "identity-file+'.pub'" {
				PublicKey = IdentityFile + ".pub"
			}

			if len(args) == 0 {
				awsSession := NewAwsSession()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				instances, err := GetRunningInstances(ctx, awsSession)
				if err != nil {
					return err
				}

				prompt := promptui.Select{
					Label: "Instances",
					Templates: &promptui.SelectTemplates{
						Label:    `{{ . | green }}`,
						Active:   `{{ ">" | blue }} {{ .ID | red }} {{ .TagName | red }}`,
						Inactive: `{{ .ID | cyan }} {{ .TagName | cyan }}`,
						Selected: `{{ .ID | yellow }} {{ .TagName | yellow }}`,
					},
					Items: instances,
					Size:  50,
					Searcher: func(input string, index int) bool {
						item := instances[index]
						instanceName := strings.Replace(strings.ToLower(item.TagName), " ", "", -1)
						instanceID := strings.Replace(strings.ToLower(item.ID), " ", "", -1)
						input = strings.Replace(strings.ToLower(input), " ", "", -1)
						if strings.Contains(instanceName, input) {
							return true
						} else if strings.Contains(instanceID, input) {
							return true
						}
						return false
					},
					StartInSearchMode: true,
				}

				index, _, err := prompt.Run()
				if err != nil {
					return err
				}

				InstanceID = instances[index].ID
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := execCmdRoot(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}

	cobra.OnInitialize(initConfig)
	cmd.Flags().StringVarP(&Username, "username", "u", "ec2-user", "")
	cmd.Flags().StringVarP(&IdentityFile, "identity-file", "i", "~/.ssh/id_rsa", "")
	cmd.Flags().StringVarP(&PublicKey, "publickey", "P", "identity-file+'.pub'", "")
	cmd.Flags().StringVarP(&Port, "port", "p", "22", "")
	cmd.Flags().StringVarP(&ExternalCommand, "external-command", "c", "", "")
	return cmd
}

func execCmdRoot(cmd *cobra.Command, args []string) error {
	// create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// fetch empty port
	localPort, err := fetchEmptyPort()
	if err != nil {
		return err
	}

	// instance-id
	if len(args) == 1 {
		InstanceID = args[0]
	}

	awsSession := NewAwsSession()
	tokens, sessionManagerParam, err := GetSsmSessionToken(ctx, awsSession, InstanceID, Port, localPort)
	if err != nil {
		return err
	}

	region := GetRegion(awsSession)
	ssmUrl := GetSsmApiUrl(region)

	cmdSessionManager, err := ExecSessionManagerPortForwarding(ctx, tokens, region, sessionManagerParam, ssmUrl)
	go func() {
		err = cmdSessionManager.Wait()
	}()

	if err = SendSSHPublicKey(ctx, awsSession, InstanceID, Username, PublicKey); err != nil {
		return err
	}

	err = ExecSshLogin(Username, localPort, IdentityFile)

	//time.Sleep(5 * time.Second)

	return err
}
