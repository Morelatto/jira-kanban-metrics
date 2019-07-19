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

func getIssuesBetweenInProjectWithStatus(start, end string, project string, statuses []string) string {
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

func countWeekDays(start, end time.Time) int {
	var weekDays = 0

	dateIndex := start
	for dateIndex.Before(end) || dateIndex.Equal(end) {
		if dateIndex.Weekday() != time.Saturday && dateIndex.Weekday() != time.Sunday {
			weekDays++
		}
		dateIndex = dateIndex.AddDate(0, 0, 1)
	}

	return weekDays
}

func countWeekendDays(start time.Time, end time.Time) int {
	var weekendDays = 0

	if start.IsZero() {
		return -1
	}

	dateIndex := start
	for dateIndex.Before(end) || dateIndex.Equal(end) {
		if dateIndex.Weekday() == time.Saturday || dateIndex.Weekday() == time.Sunday {
			weekendDays++
		}
		dateIndex = dateIndex.AddDate(0, 0, 1)
	}

	return weekendDays
}

func subtractDatesRemovingWeekends(start time.Time, end time.Time) time.Duration {
	statusChangeDuration := end.Sub(start)
	weekendDaysBetweenDates := countWeekendDays(start, end)
	if weekendDaysBetweenDates > 0 {
		updatedTotalSeconds := statusChangeDuration.Seconds() - float64(60*60*24*weekendDaysBetweenDates)
		statusChangeDuration = time.Duration(updatedTotalSeconds) * time.Second
	}
	return statusChangeDuration
}

func formatColumns(columns []string) string {
	str := ""

	for index, col := range columns {
		str += "'" + col + "'"
		if index < len(columns)-1 {
			str += ","
		}
	}

	return str
}

func containsStatus(statuses []string, status string) bool {
	for _, s := range statuses {
		if strings.ToUpper(s) == strings.ToUpper(status) {
			return true
		}
	}

	return false
}
