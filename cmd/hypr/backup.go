package hypr

import (
	"github.com/Seann-Moser/hypr-config-manager/pkg/configfinder"
	"github.com/spf13/cobra"
)

type BackupConfig struct {
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "A brief description of your command",
	Long:  ``,

	RunE: func(cmd *cobra.Command, args []string) error {
		cfgFinder, err := configfinder.NewConfigFinder()
		if err != nil {
			return err
		}
		files, err := cfgFinder.FindConfigFiles("hyprland")
		if err != nil {
			return err
		}
		for _, file := range files {
			println(file)
		}
		/*
		   todo:

		   get a list of programs in config
		   ignore custom.conf
		   check if all programs are valid
		   log which ones are not
		   if not say to put in an issue on the github page


		*/
		return nil
	},
}

func setBackupFlags(cmd *cobra.Command) error {
	return nil
}
