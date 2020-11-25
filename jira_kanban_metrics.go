/*
   jira-kanban-metrics - Small application to extract Kanban metrics from a Jira project
   Copyright (C) 2015 Fausto Santos <fstsantos@gmail.com>

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"fmt"
	"github.com/andygrunwald/go-jira"
	"github.com/docopt/docopt-go"
	"log"
	"sort"
	"time"
)

var usage = `Jira kanban metrics

Usage: 
  jira-kanban-metrics <start> <end> [--debug]
  jira-kanban-metrics <JQL> [--debug]
  jira-kanban-metrics -h | --help
  jira-kanban-metrics --version

Arguments:
  start  Start date in dd/mm/yyyy format.
  end    End date in dd/mm/yyyy format.
  JQL    The jql.

Options:
  --debug    Print debug output.
  -h --help  Show this screen.
  --version  Show version.
`

const version = "1.4"

func main() {
	arguments, _ := docopt.ParseArgs(usage, nil, version)
	err := arguments.Bind(&CLParameters)
	if err != nil {
		log.Fatalf("Failed to parse command line arguments: %v", err)
	}

	loadBoardCfg()
	authJiraClient()

	title("Extracting Kanban metrics from project %s // ", BoardCfg.Project)
	title("From %s to %s\n", CLParameters.StartDate, CLParameters.EndDate)

	var issues []jira.Issue
	if CLParameters.Jql != "" {
		issues = searchIssues(CLParameters.Jql)
	} else {
		issues = searchIssues(getIssuesJqlSearch())
	}

	startDate, endDate := parseDate(CLParameters.StartDate), parseDate(CLParameters.EndDate)

	issueDetails := getIssueDetailsList(issues, endDate)

	printNotMapped(issueDetails)

	byType := getIssueDetailsMapByType(issueDetails)
	printIssueDetailsByType(byType)
	printAverageByStatus(issueDetails)
	printAverageByStatusType(issueDetails)
	printWIP(issueDetails, countWeekDays(startDate, endDate))
	printThroughput(issueDetails)
	printLeadTime(byType)
}

func printNotMapped(issueDetails []IssueDetails) {
	if CLParameters.Debug {
		notMapped := getNotMapped(issueDetails)
		if len(notMapped) > 0 {
			warn("\nThe following status were found but not mapped in board.cfg:\n")
			for status, _ := range notMapped {
				fmt.Println(status)
			}
		}
	}
}

func getIssueDetailsList(issues []jira.Issue, endDate time.Time) []IssueDetails {
	var issueDetailsList []IssueDetails
	// Add one day to end date limit to include it in time comparisons
	endDate = endDate.Add(time.Hour * time.Duration(24))
	for _, issue := range issues {
		Debug(issue.Key)

		issueDetails := IssueDetails{
			Key:          issue.Key,
			Title:        issue.Fields.Summary,
			Description:  issue.Fields.Description,
			CreatedDate:  time.Time(issue.Fields.Created),
			IssueType:    issue.Fields.Type.Name,
			Labels:       issue.Fields.Labels,
			CustomFields: getCustomFields(issue),
		}

		previousTransition := &TransitionDetails{
			Timestamp: issueDetails.CreatedDate,
			StatusTo:  "Open",
		}
		if CLParameters.Debug {
			previousTransition.PrintTo()
		}
		// sorting because history order changes on different jira versions
		sort.Sort(ByCreatedDate(issue.Changelog.Histories))
		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				transitionTime, _ := history.CreatedTime()
				if item.Field == "status" {
					if !endDate.IsZero() && transitionTime.Before(endDate) {
						var t = TransitionDetails{
							Timestamp:          transitionTime,
							StatusFrom:         item.FromString,
							StatusTo:           item.ToString,
							PreviousTransition: previousTransition,
						}

						if CLParameters.Debug {
							t.PrintTo()
						}

						previousTransition = &t

						if containsStatus(BoardCfg.DoneStatus, t.StatusTo) {
							issueDetails.ResolvedDate = t.Timestamp
						} else if issueDetails.WipDate.IsZero() && containsStatus(BoardCfg.WipStatus, t.StatusTo) {
							issueDetails.WipDate = t.Timestamp
						}
					}
				} else if item.Field == "Epic Link" {
					issueDetails.EpicLink = item.ToString
				} else if item.Field == "Sprint" {
					issueDetails.Sprint = item.ToString
				} else if item.Field == "Flagged" {
					if item.ToString != "" {
						flagDetails := FlagDetails{FlagStart: transitionTime}
						issueDetails.FlagDetails = append(issueDetails.FlagDetails, flagDetails)
					} else if item.FromString != "" && len(issueDetails.FlagDetails) != 0 {
						flagDetails := &issueDetails.FlagDetails[len(issueDetails.FlagDetails)-1]
						flagDetails.FlagEnd = transitionTime
					}
				}
			}
		}
		issueDetails.TransitionDetails = previousTransition
		issueDetailsList = append(issueDetailsList, issueDetails)
	}
	return issueDetailsList
}

type ByCreatedDate []jira.ChangelogHistory

func (c ByCreatedDate) Len() int {
	return len(c)
}

func (c ByCreatedDate) Less(i, j int) bool {
	return parseTime(c[i].Created).Before(parseTime(c[j].Created))
}

func (c ByCreatedDate) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func getIssueDetailsMapByType(issueDetails []IssueDetails) map[string][]IssueDetails {
	issueDetailsByType := make(map[string][]IssueDetails)
	for _, issueDetail := range issueDetails {
		issueDetailsByType[issueDetail.IssueType] = append(issueDetailsByType[issueDetail.IssueType], issueDetail)
	}
	return issueDetailsByType
}

func getNotMapped(issueDetails []IssueDetails) map[string]int {
	var notMapped = make(map[string]int)
	for _, issueDetail := range issueDetails {
		var currentTransition = issueDetail.TransitionDetails
		for {
			if currentTransition.StatusTo != "Open" && statusIsNotMapped(currentTransition.StatusTo) {
				notMapped[currentTransition.StatusTo]++
			}
			if currentTransition.PreviousTransition == nil {
				break
			}
			currentTransition = currentTransition.PreviousTransition
		}
	}
	return notMapped
}
