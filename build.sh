#!/bin/bash
go build -o jira-kanban-metrics -v jira-kanban-metrics.go jira.go jira-board.go parsing.go struct.go
