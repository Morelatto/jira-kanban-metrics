package main

import (
	"github.com/andygrunwald/go-jira"
	"time"
)

var CLParameters struct {
	StartDate string `docopt:"<start>"`
	EndDate   string `docopt:"<end>"`
	Jql       string `docopt:"<JQL>"`
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
	Key               string
	Title             string
	CreatedDate       time.Time
	ToWipDate         time.Time
	ResolvedDate      time.Time
	WIPIdle           time.Duration
	WIP               time.Duration
	DurationByStatus  map[string]time.Duration
	EpicLink          string
	IssueType         string
	Resolved          bool
	Sprint            string
	Labels            []string
	CustomFields      []string
	LastStatus        string
	TransitionDetails *TransitionDetails
}

func New(issue jira.Issue) IssueDetails {
	return IssueDetails{
		Key:              issue.Key,
		Title:            issue.Fields.Summary,
		CreatedDate:      time.Time(issue.Fields.Created),
		DurationByStatus: make(map[string]time.Duration),
		IssueType:        issue.Fields.Type.Name,
		Labels:           issue.Fields.Labels,
		CustomFields:     getCustomFields(issue),
	}
}
