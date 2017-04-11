#!/bin/bash

# Linux build
go build -o jira-kanban-metrics -v jira-kanban-metrics.go jira.go jira-board.go parsing.go struct.go

# Windows build
GOOS=windows GOARCH=386 go build -o jira-kanban-metrics.exe -v jira-kanban-metrics.go jira.go jira-board.go parsing.go struct.go
