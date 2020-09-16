package main

import (
	"crypto/tls"
	"fmt"
	"github.com/andygrunwald/go-jira"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
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
	jqlSearch := fmt.Sprintf("project = '%v' AND  issuetype != Epic AND status CHANGED DURING('%v', '%v') AND status NOT IN (%v)",
		BoardCfg.Project,
		formatJiraDate(parseDate(CLParameters.StartDate)),
		formatJiraDate(parseDate(CLParameters.EndDate)),
		formatColumns(BoardCfg.DoneStatus))
	if CLParameters.Debug {
		title("Not Done JQL: %s\n", jqlSearch)
	}
	return jqlSearch
}

func searchIssues(jql string) []jira.Issue {
	log.Printf("JQL: %v", jql)
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
	log.Printf("Total issues returned: %v", len(issues))
	return issues
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
		warn("Failed to get custom fields for %s\n", issueId)
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
