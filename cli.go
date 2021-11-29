package awssh

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/k1LoW/duration"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/youyo/awsprofile"
)

func Run(cmd *cobra.Command, args []string) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	profile := viper.GetString("profile")
	cache := viper.GetBool("cache")
	disableSnapshot := viper.GetBool("disable-snapshot")
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

	// Get snapshot
	if disableSnapshot != true {
		go func() {
			if imageId, err := createAMI(ctx, awsSession, instanceID); err != nil {
				fmt.Printf("Failed to create to auto snapshot. error: %T\n", err)
			} else {
				fmt.Println("Create AMI ID: " + *imageId)
			}
		}()
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

	if err = sendSSHPublicKey(ctx, awsSession, instanceID, viper.GetString("username"), viper.GetString("publicKey")); err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	cmdSsh, err := execSshCommand(ctx, viper.GetString("username"), ConnectHost, localPort, viper.GetString("identity-file"))
	cmdSsh.Wait()

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
	if err = checkSessionManagerCommandIsExist(); err != nil {
		return err
	}

	guessedPublickey := guessPublickey(
		viper.GetString("identity-file"),
		viper.GetString("publickey"),
	)
	viper.Set("publickey", guessedPublickey)

	selectProfile := viper.GetBool("select-profile")
	if selectProfile {
		awsProfile := awsprofile.New()

		if err := awsProfile.Parse(); err != nil {
			return err
		}

		profiles, err := awsProfile.ProfileNames()
		if err != nil {
			return err
		}

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
