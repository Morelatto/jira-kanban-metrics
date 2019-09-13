package main

import (
	"crypto/tls"
	"fmt"
	"github.com/andygrunwald/go-jira"
	"io/ioutil"
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

func getDoneIssuesJqlSearch() string {
	jqlSearch := fmt.Sprintf("project = '%v' AND issuetype != Epic AND (status CHANGED TO (%v) DURING('%v', '%v'))",
		BoardCfg.Project,
		formatColumns(BoardCfg.DoneStatus),
		formatJiraDate(parseDate(CLParameters.StartDate)),
		formatJiraDate(parseDate(CLParameters.EndDate)))
	if CLParameters.Debug {
		title("WIP/Throughput JQL: %s\n", jqlSearch)
	}
	return jqlSearch
}

func getNotDoneIssuesJqlSearch() string {
	jqlSearch := fmt.Sprintf("project = '%v' AND  issuetype != Epic AND status CHANGED TO (%v) DURING('%v', '%v') AND status NOT IN (%v)",
		BoardCfg.Project,
		formatColumns(append(append(BoardCfg.OpenStatus, BoardCfg.WipStatus...), BoardCfg.IdleStatus...)),
		formatJiraDate(parseDate(CLParameters.StartDate)),
		formatJiraDate(parseDate(CLParameters.EndDate)),
		formatColumns(BoardCfg.DoneStatus))
	if CLParameters.Debug {
		title("Not Done JQL: %s\n", jqlSearch)
	}
	return jqlSearch
}

func searchIssues(jql string) []jira.Issue {
	searchOptions := &jira.SearchOptions{
		MaxResults: 1000,
		Expand:     "changelog",
	}
	issues, response, err := JiraClient.Issue.Search(jql, searchOptions)
	if err != nil {
		panic(err)
	}
	if response.StatusCode != 200 {
		fmt.Println("Response Code: " + response.Status)
		bodyBytes, _ := ioutil.ReadAll(response.Body)
		fmt.Println("Body: " + string(bodyBytes))
		return nil
	}
	return issues
}

func getIssueDetails(issue jira.Issue) IssueDetails {
	return IssueDetails{
		Name:             issue.Key,
		Summary:          issue.Fields.Summary,
		CreatedDate:      time.Time(issue.Fields.Created),
		DurationByStatus: make(map[string]time.Duration),
		IssueType:        issue.Fields.Type.Name,
		Labels:           issue.Fields.Labels,
		CustomFields:     getCustomFields(issue),
	}
}

func getCustomFields(issue jira.Issue) []string {
	var customFields []string
	if len(BoardCfg.CustomFields) != 0 {
		for _, custom := range BoardCfg.CustomFields {
			value := getCustomFieldValue(custom, issue.ID)
			if value != "" {
				customFields = append(customFields, value)
			}
		}
	}
	return customFields
}

func getCustomFieldValue(customField, issueId string) string {
	fields, res, err := JiraClient.Issue.GetCustomFields(issueId)
	if err != nil {
		return ""
	}
	if res.StatusCode != 200 {
		fmt.Println("Response Code: " + res.Status)
		bodyBytes, _ := ioutil.ReadAll(res.Body)
		fmt.Println("Body: " + string(bodyBytes))
	} else {
		for name, value := range fields {
			if name == customField && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}
