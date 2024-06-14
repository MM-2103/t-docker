package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

// Initialize command
func init() {
	rootCmd.AddCommand(psCmd)
}

// Command defention
var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List docker containers",
	Run: func(cmd *cobra.Command, args []string) {
		cmda := exec.Command("docker", "ps")
		stdout, err := cmda.Output()

		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println(string(stdout))
	},
}
