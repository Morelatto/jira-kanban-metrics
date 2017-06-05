package main

import (
    "os"
    "fmt"
    "encoding/json"
)

func loadBoardCfg() BoardCfg {
    if _, err := os.Stat("jira_board.cfg"); os.IsNotExist(err) {
        panic("jira_board.cfg not found")
    }

    file, _ := os.Open("jira_board.cfg")
    decoder := json.NewDecoder(file)
    boardCfg := BoardCfg{}
    err := decoder.Decode(&boardCfg)

    if err != nil {
        fmt.Println("error:", err)
    }

    return boardCfg
}
