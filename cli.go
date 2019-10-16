package awssh

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/k1LoW/duration"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func Run(cmd *cobra.Command, args []string) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	profile := viper.GetString("profile")
	cache := viper.GetBool("cache")
	duration, err := duration.Parse(viper.GetString("duration"))
	if err != nil {
		return err
	}

	awsSession := newAwsSession(profile, cache, duration)

	var instanceID string

	if len(args) == 1 {
		instanceID = args[0]
	} else {
		instances, err := getRunningInstances(ctx, awsSession)
		if err != nil {
			return err
		}

		instanceID, err = selectInstance(instances)
		if err != nil {
			return err
		}
	}

	// fetch empty port
	localPort, err := fetchEmptyPort(ConnectHost)
	if err != nil {
		return err
	}

	tokens, sessionManagerParam, err := getSsmSessionToken(ctx, awsSession, instanceID, viper.GetString("port"), localPort)
	if err != nil {
		return err
	}

	region := getRegion(awsSession)
	ssmUrl := getSsmApiUrl(region)

	cmdPortForwarding, err := execSessionManagerPortForwarding(ctx, tokens, region, sessionManagerParam, ssmUrl)
	if err != nil {
		return err
	}
	defer cmdPortForwarding.Process.Kill()

	waitOpenPort(ConnectHost, localPort)

	if err = sendSSHPublicKey(ctx, awsSession, instanceID, viper.GetString("username"), viper.GetString("publicKey")); err != nil {
		return err
	}

	err = ExecSshLogin(viper.GetString("username"), ConnectHost, localPort, viper.GetString("identity-file"))

	return err
}

func Validate(cmd *cobra.Command, args []string) (err error) {
	if len(args) > 1 {
		err = errors.New("accepts only 1 arg")
		return err
	}

	if len(args) == 1 {
		instanceIdRe := regexp.MustCompile(
			`^i-([a-zA-Z0-9A0-zZ9]{8}|[a-zA-Z0-9A0-zZ9]{17})$`,
		)
		if !instanceIdRe.MatchString(args[0]) {
			err = errors.New("unmatched instance-id")
			return err
		}
	}

	return nil
}

func PreRun(cmd *cobra.Command, args []string) (err error) {
	guessedPublickey := guessPublickey(
		viper.GetString("identity-file"),
		viper.GetString("publickey"),
	)
	viper.Set("publickey", guessedPublickey)

	selectProfile := viper.GetBool("select-profile")
	if selectProfile {
		c := NewConfig()
		if err := c.Load(); err != nil {
			return err
		}

		profiles := c.ListProfiles()

		prompt := promptui.Select{
			Label: "Profiles",
			Templates: &promptui.SelectTemplates{
				Label:    `{{ . | green }}`,
				Active:   `{{ ">" | blue }} {{ . | red }}`,
				Inactive: `{{ . | cyan }}`,
				Selected: `{{ . | yellow }}`,
			},
			Items: profiles,
			Size:  25,
			Searcher: func(input string, index int) bool {
				item := profiles[index]
				profileName := strings.Replace(strings.ToLower(item), " ", "", -1)
				input = strings.Replace(strings.ToLower(input), " ", "", -1)
				if strings.Contains(profileName, input) {
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

		viper.Set("profile", profiles[index])
	}

	return nil
}
