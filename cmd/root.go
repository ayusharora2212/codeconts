package cmd

import (
	"os"

	checkmyrepoCmd "github.com/fossMeDaddy/codeconts/src-cmd/checkmyrepo"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "codeconts",
	Short: "CodeConts is a tool for telling the code contributions of the developers",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	rootCmd.AddCommand(checkmyrepoCmd.Init())
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
