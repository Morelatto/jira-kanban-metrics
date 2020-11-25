package main

import (
	"crypto/tls"
	"fmt"
	"github.com/andygrunwald/go-jira"
	"github.com/zchee/color"
	"log"
	"net/http"
	"strings"
	"time"
)

var JiraClient jira.Client

func authJiraClient() {
	tp := jira.BasicAuthTransport{
		Username: strings.TrimSpace(BoardCfg.Login),
		Password: strings.TrimSpace(BoardCfg.Password),
		// ignore missing certs
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	client, err := jira.NewClient(tp.Client(), BoardCfg.JiraUrl)
	if err != nil {
		panic(err)
	}

	JiraClient = *client
}

const issuesJql = "project = '%v' AND issuetype != Epic AND status CHANGED DURING('%v', '%v') ORDER BY status"

func getIssuesJqlSearch() string {
	jqlSearch := fmt.Sprintf(issuesJql, BoardCfg.Project, formatJiraDate(parseDate(CLParameters.StartDate)), formatJiraDate(parseDate(CLParameters.EndDate)))
	if CLParameters.Debug {
		title("JQL: %s\n", jqlSearch)
	}
	return jqlSearch
}

func searchIssues(jql string) []jira.Issue {
	if CLParameters.Debug {
		log.Printf("JQL: %v", jql)
	}
	var i = 0
	var issues []jira.Issue
	searchOptions := jira.SearchOptions{MaxResults: 100, Expand: "changelog"}
	for {
		searchOptions.StartAt = i
		res, resp, err := JiraClient.Issue.Search(jql, &searchOptions)
		if err != nil {
			log.Fatalf("Failed to search issues on jira: %v\nResponse body: %v", err, readResponseBody(resp))
		}
		issues = append(issues, res...)
		i += resp.MaxResults
		if i >= resp.Total {
			break
		}
	}
	if CLParameters.Debug {
		log.Printf("Total issues returned: %v", len(issues))
	}
	return issues
}

type CustomField interface {
	Id() string
	String() string
	Unmarshall(interface{}) CustomField
}

type SprintCustomField struct {
	Name         string
	State        string
	StartDate    time.Time
	EndDate      time.Time
	CompleteDate time.Time
}

func (SprintCustomField) Id() string {
	return "customfield_10021"
}

func (cf SprintCustomField) String() string {
	return color.CyanString(cf.Name)
}

func (SprintCustomField) Unmarshall(data interface{}) CustomField {
	cf := SprintCustomField{}
	m := data.(map[string]interface{})
	if name, ok := m["name"].(string); ok {
		cf.Name = name
	}
	if state, ok := m["state"].(string); ok {
		cf.State = state
	}
	if startDateStr, ok := m["startDate"].(string); ok {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			cf.StartDate = startDate
		}
	}
	if endDateStr, ok := m["endDate"].(string); ok {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			cf.EndDate = endDate
		}
	}
	if completedDateStr, ok := m["endDate"].(string); ok {
		if completedDate, err := time.Parse(time.RFC3339, completedDateStr); err == nil {
			cf.CompleteDate = completedDate
		}
	}
	return cf
}

type FlagCustomField struct {
	Value string
}

func (FlagCustomField) Id() string {
	return "customfield_10035"
}

func (cf FlagCustomField) String() string {
	return color.RedString("Flag")
}

func (FlagCustomField) Unmarshall(data interface{}) CustomField {
	cf := FlagCustomField{}
	m := data.(map[string]interface{})
	if value, ok := m["value"].(string); ok {
		cf.Value = value
	}
	return cf
}

var supportedCustomFields = []CustomField{SprintCustomField{}, FlagCustomField{}}

func getCustomFields(issue jira.Issue) []CustomField {
	var customFields []CustomField
	for name, value := range issue.Fields.Unknowns {
		for _, supportedCustomField := range supportedCustomFields {
			if name == supportedCustomField.Id() && value != nil {
				switch customField := value.(type) {
				case []interface{}:
					for _, field := range customField {
						customFields = append(customFields, supportedCustomField.Unmarshall(field))
					}
				case interface{}:
					customFields = append(customFields, supportedCustomField.Unmarshall(customField))
				}
			}
		}
	}
	return customFields
}
