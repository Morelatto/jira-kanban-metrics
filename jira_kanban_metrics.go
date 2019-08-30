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
	"time"
)

var usage = `Jira kanban metrics

Usage: 
  jira-kanban-metrics <startDate> <endDate> [--debug]
  jira-kanban-metrics -h | --help
  jira-kanban-metrics --version

Arguments:
  startDate  Start date in dd/mm/yyyy format.
  endDate    End date in dd/mm/yyyy format.

Options:
  --debug    Print debug output [default: false].
  -h --help  Show this screen.
  --version  Show version.
`

func main() {
	arguments, _ := docopt.ParseArgs(usage, nil, "1.0")
	err := arguments.Bind(&CLParameters)
	if err != nil {
		panic(err)
	}

	loadBoardCfg()
	authJiraClient()

	jqlSearch := getJqlSearch(CLParameters.StartDate, CLParameters.EndDate, BoardCfg.Project, BoardCfg.DoneStatus)
	issues := searchIssues(jqlSearch)
	wipStatus := append(BoardCfg.WipStatus, BoardCfg.IdleStatus...)
	extractMetrics(issues, wipStatus)
}

func extractMetrics(issues []jira.Issue, wipStatus []string) {
	title("Extracting Kanban metrics from project %s, %s to %s\n", BoardCfg.Project, CLParameters.StartDate, CLParameters.EndDate)

	var totalWipDuration time.Duration
	var issueDetailsMapByType = make(map[string][]IssueDetails)
	var totalDurationByStatusMap = make(map[string]time.Duration)
	var notMappedStatus []string

	// Transitions on the board: Issue -> Changelog -> Histories -> Items -> Field:Status
	for _, issue := range issues {
		var issueDetails IssueDetails
		var lastToStatus string
		var wipTransitionTime time.Time
		var lastTransitionTime = time.Time(issue.Fields.Created)

		if CLParameters.Debug {
			title("\n%s\n", issue.Key)
		}

		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				// Ignore transitions to the same status
				if item.From == item.To {
					continue
				}

				if item.Field == "status" {
					// Timestamp when the transition happened
					transitionTime, _ := history.CreatedTime()

					// Mapping var to calculate total WIP of the issue
					if wipTransitionTime.IsZero() && containsStatus(wipStatus, item.ToString) {
						wipTransitionTime = transitionTime
					}

					// Calculates time difference between transitions subtracting weekend days
					transitionDuration := transitionTime.Sub(lastTransitionTime)
					weekendDays := countWeekendDays(lastTransitionTime, transitionTime)
					if weekendDays > 0 {
						transitionDuration -= time.Duration(weekendDays) * time.Hour * 24
					}

					// Adding it to the total count only if in WIP/Idle
					if containsStatus(wipStatus, item.FromString) {
						totalWipDuration += transitionDuration
						issueDetails.WIP += transitionDuration
						totalDurationByStatusMap[item.FromString] += transitionDuration
					}

					// Check if status is not mapped on cfg to warn the user
					if !containsStatus(notMappedStatus, item.FromString) && statusIsNotMapped(item.FromString) {
						notMappedStatus = append(notMappedStatus, item.FromString)
					}

					// Update vars for next iteration
					lastToStatus = item.ToString
					lastTransitionTime = transitionTime
				} else if item.Field == "Epic Link" {
					issueDetails.EpicLink = item.ToString
				} else if item.Field == "Sprint" {
					issueDetails.Sprint = item.ToString
				}
			}
		}

		issueDetails.Name = issue.Key
		issueDetails.Summary = issue.Fields.Summary
		if wipTransitionTime.IsZero() {
			issueDetails.StartDate = time.Time(issue.Fields.Created)
		} else {
			issueDetails.StartDate = wipTransitionTime
		}
		issueDetails.EndDate = lastTransitionTime
		issueDetails.IssueType = issue.Fields.Type.Name
		issueDetails.Resolved = containsStatus(BoardCfg.DoneStatus, lastToStatus)
		issueDetails.Labels = issue.Fields.Labels

		issueDetailsMapByType[issueDetails.IssueType] = append(issueDetailsMapByType[issueDetails.IssueType], issueDetails)
	}

	if len(notMappedStatus) > 0 {
		warn("\nThe following status were found but not mapped in board.cfg:\n")
		for _, status := range notMappedStatus {
			fmt.Println(status)
		}
	}

	printIssueDetailsByType(issueDetailsMapByType)
	printAverageByStatus(totalDurationByStatusMap, totalWipDuration)
	printAverageByStatusType(totalDurationByStatusMap, totalWipDuration)
	printWIP(totalWipDuration, len(issues), countWeekDays(parseDate(CLParameters.StartDate), parseDate(CLParameters.EndDate)))
	printThroughput(len(issues), issueDetailsMapByType)
	printLeadTime(totalWipDuration, len(issues), issueDetailsMapByType)
	// TODO print scaterplot
}
