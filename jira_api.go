package main

import (
	"crypto/tls"
	"fmt"
	"github.com/andygrunwald/go-jira"
	"io/ioutil"
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

func getJqlSearch(start, end string, project string, statuses []string) string {
	jqlSearch := fmt.Sprintf("project = '%v' AND  issuetype != Epic AND (status CHANGED TO (%v) DURING('%v', '%v'))",
		project, formatColumns(statuses), formatJiraDate(parseDate(start)), formatJiraDate(parseDate(end)))
	if CLParameters.Debug {
		title("WIP/Throughput JQL: %s\n", jqlSearch)
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
