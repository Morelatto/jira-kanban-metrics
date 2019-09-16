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
	"github.com/hako/durafmt"
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

	wipStatus := append(BoardCfg.WipStatus, BoardCfg.IdleStatus...)
	doneIssues := searchIssues(getDoneIssuesJqlSearch())
	notDoneIssues := searchIssues(getNotDoneIssuesJqlSearch())

	title("Extracting Kanban metrics from project %s // ", BoardCfg.Project)
	title("From %s to %s\n", CLParameters.StartDate, CLParameters.EndDate)

	startDate, endDate := parseDate(CLParameters.StartDate), parseDate(CLParameters.EndDate)
	// Add one day to end date limit to include it in time comparisons
	endDate = endDate.Add(time.Hour * time.Duration(24))

	doneIssuesByTypeMap := extractMetrics(doneIssues, wipStatus)
	notDoneIssuesByTypeMap := extractMetrics(notDoneIssues, wipStatus)

	printIssueDetailsByType(mergeMaps(doneIssuesByTypeMap, notDoneIssuesByTypeMap))
	printAverageByStatus(doneIssuesByTypeMap)
	printAverageByStatusType(doneIssuesByTypeMap)
	printWIP(doneIssuesByTypeMap, countWeekDays(startDate, endDate))
	printThroughput(doneIssuesByTypeMap)
	printLeadTime(doneIssuesByTypeMap)
	// TODO print scaterplot
}

func extractMetrics(issues []jira.Issue, wipStatus []string) map[string][]IssueDetails {
	var issueDetailsMapByType = make(map[string][]IssueDetails)
	var notMappedStatus []string

	// Transitions on the board: Issue -> Changelog -> Histories -> Items -> Field:Status
	for _, issue := range issues {
		issueDetails := getIssueDetails(issue)

		issueCreationTime := time.Time(issue.Fields.Created)
		wipTransitionTime := issueCreationTime
		lastTransitionTime := issueCreationTime

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

					if CLParameters.Debug {
						info("%s -> %s\n", item.FromString, item.ToString)
						info("%s -> %s", formatBrDateWithTime(lastTransitionTime), formatBrDateWithTime(transitionTime))
					}

					transitionDuration := getTransitionDuration(lastTransitionTime, transitionTime)

					if CLParameters.Debug {
						warn(" [%s]", durafmt.Parse(transitionDuration))
					}

					// Check if status is not mapped on cfg to warn the user
					if !containsStatus(notMappedStatus, item.ToString) && statusIsNotMapped(item.ToString) {
						notMappedStatus = append(notMappedStatus, item.ToString)
					}

					// Update vars for next iteration
					lastTransitionTime = transitionTime
					issueDetails.LastStatus = item.ToString

					if containsStatus(wipStatus, item.FromString) || containsStatus(BoardCfg.DoneStatus, item.ToString) {
						// Mapping var to calculate total WIP of the issue
						if wipTransitionTime == issueCreationTime {
							wipTransitionTime = transitionTime
						}
						// Mapping only WIP transitions to use for metrics later
						issueDetails.WIP += transitionDuration
						issueDetails.DurationByStatus[item.FromString] += transitionDuration
					}

					if containsStatus(BoardCfg.DoneStatus, item.ToString) {
						issueDetails.Resolved = true
						issueDetails.ResolvedDate = transitionTime
					}

					warn("\n")
				} else if item.Field == "Epic Link" {
					issueDetails.EpicLink = item.ToString
				} else if item.Field == "Sprint" {
					issueDetails.Sprint = item.ToString
				}
			}
		}

		// Consider WIP date until now if last status on WIP
		if !issueDetails.Resolved && containsStatus(wipStatus, issueDetails.LastStatus) {
			transitionDuration := getTransitionDuration(lastTransitionTime, time.Now())
			issueDetails.WIP += transitionDuration
			issueDetails.DurationByStatus[issueDetails.LastStatus] += transitionDuration
		}

		issueDetails.ToWipDate = wipTransitionTime
		issueDetailsMapByType[issueDetails.IssueType] = append(issueDetailsMapByType[issueDetails.IssueType], issueDetails)
	}

	if len(notMappedStatus) > 0 {
		warn("\nThe following status were found but not mapped in board.cfg:\n")
		for _, status := range notMappedStatus {
			fmt.Println(status)
		}
	}
	return issueDetailsMapByType
}

// Calculates time difference between transitions subtracting weekend days
func getTransitionDuration(firstTransition time.Time, secondTransition time.Time) time.Duration {
	transitionDuration := secondTransition.Sub(firstTransition)
	weekendDays := countWeekendDays(firstTransition, secondTransition)
	if weekendDays > 0 {
		if getDays(transitionDuration) >= weekendDays {
			transitionDuration -= time.Duration(weekendDays) * time.Hour * 24
		} else {
			transitionDuration = 0
		}
	}
	return transitionDuration
}
