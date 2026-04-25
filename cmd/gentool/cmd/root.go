package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version is set at build time via -ldflags="-X 'github.com/ny4rl4th0t3p/cosmos-genesis-tool/cmd/gentool/cmd.Version=<tag>'".
var Version = "dev"

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "gentool",
	Version: Version,
	Short:   "Generate a Cosmos SDK genesis file from CSV inputs",
	Long:    `gentool builds a genesis file for any Cosmos SDK chain from a baseline genesis and CSV-defined accounts, claims, and grants.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./.gentool.yaml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gentool")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
