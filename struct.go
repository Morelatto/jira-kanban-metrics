package main

import (
	"time"
)

var CLParameters struct {
	StartDate string `docopt:"<startDate>"`
	EndDate   string `docopt:"<endDate>"`
	Debug     bool
}

var BoardCfg struct {
	JiraUrl      string
	Login        string
	Password     string
	Project      string
	OpenStatus   []string
	WipStatus    []string
	IdleStatus   []string
	DoneStatus   []string
	CustomFields []string
}

type IssueDetails struct {
	Name             string
	Summary          string
	StartDate        time.Time
	EndDate          time.Time
	DurationByStatus map[string]time.Duration
	EpicLink         string
	IssueType        string
	Resolved         bool
	Sprint           string
	Labels           []string
	CustomFields     []string
}
