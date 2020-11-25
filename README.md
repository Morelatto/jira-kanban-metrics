# jira-kanban-metrics

Small application to extract Kanban metrics from a Jira project

## Requirements
* Go >= 1.11

## Installation
```
git clone https://github.com/fstsantos/jira-kanban-metrics
cd jira-kanban-metrics
go build
./jira-kanban-metrics -h
```


## Usage
```
jira-kanban-metrics <startDate> <endDate> [--debug]
jira-kanban-metrics <JQL> [--debug]
jira-kanban-metrics -h | --help
jira-kanban-metrics --version
```

## Arguments
```
startDate     Start date in dd/mm/yyyy format.
endDate       End date in dd/mm/yyyy format.
JQL           A JQL to use as input for the script.
```

## Options
```
--debug       Print debug output [default: false].
-h --help     Show this screen.
--version     Show version.
```

## Configuration

##### jira_board.cfg
```
"JiraUrl":      "http://jira.intranet/jira",
"Login":        "",
"Password":     "",
"Project":      "",
"OpenStatus":   ["BACKLOG", "OPEN"],
"WipStatus":    ["IN PROGRESS", "TEST"],
"IdleStatus":   ["DEV DONE", "TEST DONE"],
"DoneStatus":   ["DONE"]
```
