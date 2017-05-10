package main

import (
    "os"
    "fmt"
    "encoding/json"
)

func loadBoardCfg() BoardCfg {
    if _, err := os.Stat("jira-board.cfg"); os.IsNotExist(err) {
        panic("jira-board.cfg not found")
    }

    file, _ := os.Open("jira-board.cfg")
    decoder := json.NewDecoder(file)
    boardCfg := BoardCfg{}
    err := decoder.Decode(&boardCfg)

    if err != nil {
        fmt.Println("error:", err)
    }

    return boardCfg
}
