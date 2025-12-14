package configfinder

import (
	"fmt"
	"log"
	"testing"

	"github.com/Seann-Moser/hypr-config-manager/pkg/hyprconfig"
)

func TestFind(t *testing.T) {
	// Initialize the ConfigFinder
	cf, err := NewConfigFinder()
	if err != nil {
		log.Fatal(err)
	}

	// Find configuration files for a given program
	program := "hyprland"
	configFiles, err := cf.FindConfigFiles(program)
	if err != nil {
		log.Fatal(err)
	}

	// Print out the configuration files found
	if len(configFiles) > 0 {
		fmt.Println("Found configuration files for", program)
		for _, file := range configFiles {
			fmt.Println(file)
			f, _ := hyprconfig.ExtractExecOnceCommandsFile(file)
			for _, i := range f {
				fmt.Printf("\t%s\n", i)
			}

		}
	} else {
		fmt.Println("No configuration files found for", program)
	}

}

func TestFinds(t *testing.T) {
	// Initialize the ConfigFinder
	// Find configuration files for a given program
	f, _ := hyprconfig.ExtractExecOnceCommandsFile("/home/n9s/.config/hypr/hyprland.conf")
	for _, i := range f {
		fmt.Printf("\t%s\n", i)
	}

}
