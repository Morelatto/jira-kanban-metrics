package main

import (
	"encoding/json"
	"os"
)

func loadBoardCfg() {
	const configFile = "jira_board.cfg"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		panic(configFile + " not found")
	}

	file, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&BoardCfg)
	if err != nil {
		panic(err)
	}
}
