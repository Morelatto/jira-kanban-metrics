package main

import (
	"encoding/json"
	"log"
	"os"
)

const configFile = "jira_board.cfg"

func loadBoardCfg() {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		panic(configFile + " not found")
	}

	file, err := os.Open(configFile)
	defer file.Close()
	if err != nil {
		log.Fatalf("Failed to open config file %v: %v", configFile, err)
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&BoardCfg)
	if err != nil {
		log.Fatalf("Failed to decode config file %v: %v", configFile, err)
	}
}
