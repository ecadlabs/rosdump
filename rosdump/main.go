package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"git.ecadlabs.com/ecad/rostools/rosdump/config"
)

func main() {
	var (
		configFile string
	)

	flag.StringVar(&configFile, "c", "", "Config")
	flag.Parse()

	if configFile == "" {
		flag.Usage()
		os.Exit(0)
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(cfg, "", "\t")
	fmt.Println(string(b))
}
