package main

import (
	"time"
)

type CLParameters struct {
	Login        string
	StartDate    time.Time
	EndDate      time.Time
	JiraUrl      string
	Debug        bool
	DebugVerbose bool
}

type BoardCfg struct {
	Project    string
	OpenStatus []string
	WipStatus  []string
	IdleStatus []string
	DoneStatus []string
}

type IssueDetails struct {
	Name      string
	Summary   string
	StartDate time.Time
	EndDate   time.Time
	WIP       int
	EpicLink  string
	IssueType string
	Resolved  bool
	Sprint    string
	Labels    []string
}
