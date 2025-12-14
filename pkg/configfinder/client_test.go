package configfinder

//func TestFind(t *testing.T) {
//	// Initialize the ConfigFinder
//	cf, err := NewConfigFinder()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Find configuration files for a given program
//	program := "hyprland"
//	configFiles, err := cf.FindConfigFiles(program)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Print out the configuration files found
//	if len(configFiles) > 0 {
//		fmt.Println("Found configuration files for", program)
//		for _, file := range configFiles {
//			fmt.Println(file)
//			f, _ := hyprconfig.ExtractExecOnceCommandsFile(file)
//			// Verify which programs are installed
//			status := utils.VerifyPrograms(f)
//
//			// Print the installation status of each program
//			for program, installed := range status {
//				if installed {
//					fmt.Printf("%s is installed\n", program)
//				} else {
//					fmt.Printf("%s is NOT installed\n", program)
//				}
//			}
//
//		}
//	} else {
//		fmt.Println("No configuration files found for", program)
//	}
//
//}

//
//func TestFinds(t *testing.T) {
//	// Initialize the ConfigFinder
//	// Find configuration files for a given program
//	f, _ := hyprconfig.ExtractExecOnceCommandsFile("/home/n9s/.config/hypr/hyprland.conf")
//	for _, i := range f {
//		fmt.Printf("\t%s\n", i)
//	}
//
//}
