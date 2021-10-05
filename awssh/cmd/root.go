package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/youyo/awssh"
)

var Version string

var rootCmd = &cobra.Command{
	Use:          "awssh [instance-id]",
	Short:        "CLI tool to login ec2 instance.",
	Version:      Version,
	Args:         awssh.Validate,
	PreRunE:      awssh.PreRun,
	RunE:         awssh.Run,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringP("username", "u", "ec2-user", "ssh login username.")
	rootCmd.Flags().StringP("identity-file", "i", "~/.ssh/id_rsa", "identity file path.")
	rootCmd.Flags().StringP("publickey", "P", "identity-file+'.pub'", "public key file path.")
	rootCmd.Flags().StringP("port", "p", "22", "ssh login port.")
	rootCmd.Flags().StringP("external-command", "c", "", "feature use.")
	rootCmd.Flags().String("profile", "default", "use a specific profile from your credential file.")
	rootCmd.Flags().Bool("select-profile", false, "select a specific profile from your credential file.")
	rootCmd.Flags().Bool("cache", false, "enable cache a credentials.")
	rootCmd.Flags().String("duration", "1 hour", "cache duration.")
	rootCmd.Flags().Bool("disable-snapshot", false, "disable snapshot.")

	viper.BindPFlags(rootCmd.Flags())
}

func initConfig() {
	viper.BindEnv("profile", "AWS_PROFILE")
}
