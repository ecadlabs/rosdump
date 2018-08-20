package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
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

	cfg, err := LoadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(cfg, "", "\t")
	fmt.Println(string(b))
}
