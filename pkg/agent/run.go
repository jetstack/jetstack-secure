package agent

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// ConfigFilePath is where the agent will try to load the configuration from
var ConfigFilePath string

// Run starts the agent process
func Run(cmd *cobra.Command, args []string) {
	file, err := os.Open(ConfigFilePath)
	if err != nil {
		log.Fatalf("Failed to load config file for agent from: %s", ConfigFilePath)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)

	config, err := ParseConfig(b)
	if err != nil {
		log.Fatalf("Failed to parse config file: %s", err)
	}

	dump, err := config.Dump()
	if err != nil {
		log.Fatalf("Failed to dump config: %s", err)
	}
	log.Printf("Loaded config: \n%s", dump)

	for {
		log.Printf("Running Agent... TODO")
		time.Sleep(10 * time.Second)
	}
}
