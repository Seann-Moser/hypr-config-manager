package hypr

import (
	"github.com/spf13/cobra"
)

var HyprCmd = &cobra.Command{
	Use:   "hypr",
	Short: "A brief description of your command",
	Long:  ``,

	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	HyprCmd.AddCommand(backupCmd)

}

func setHyprFlags(cmd *cobra.Command) error {
	return nil
}
