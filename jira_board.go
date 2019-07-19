package main

import (
	"encoding/json"
	"os"
)

func loadBoardCfg() {
	if _, err := os.Stat("jira_board.cfg"); os.IsNotExist(err) {
		panic("jira_board.cfg not found")
	}

	file, err := os.Open("jira_board.cfg")
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
