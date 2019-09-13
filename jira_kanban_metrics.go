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

var totalWipDuration time.Duration

func main() {
	arguments, _ := docopt.ParseArgs(usage, nil, "1.0")
	err := arguments.Bind(&CLParameters)
	if err != nil {
		panic(err)
	}

	loadBoardCfg()
	authJiraClient()

	wipStatus := append(BoardCfg.WipStatus, BoardCfg.IdleStatus...)
	jqlSearch := getDoneIssuesJqlSearch(CLParameters.StartDate, CLParameters.EndDate, BoardCfg.Project, BoardCfg.DoneStatus)
	issues := searchIssues(jqlSearch)

	title("Extracting Kanban metrics from project %s // ", BoardCfg.Project)
	title("From %s to %s\n", CLParameters.StartDate, CLParameters.EndDate)
	issueDetailsMapByType := extractMetrics(issues, wipStatus)

	printIssueDetailsByType(issueDetailsMapByType)
	printAverageByStatus(issueDetailsMapByType)
	printAverageByStatusType(issueDetailsMapByType)
	printWIP(totalWipDuration, len(issues), countWeekDays(parseDate(CLParameters.StartDate), parseDate(CLParameters.EndDate)))
	printThroughput(issueDetailsMapByType)
	printLeadTime(issueDetailsMapByType)
	// TODO print scaterplot
}

func extractMetrics(issues []jira.Issue, wipStatus []string) map[string][]IssueDetails {
	var issueDetailsMapByType = make(map[string][]IssueDetails)
	var notMappedStatus []string

	startDate, endDate := parseDate(CLParameters.StartDate), parseDate(CLParameters.EndDate)
	// Add one day to end date limit to include it in time comparisons
	endDate = endDate.Add(time.Hour * time.Duration(24))

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

					// Calculates time difference between transitions subtracting weekend days
					transitionDuration := transitionTime.Sub(lastTransitionTime)
					weekendDays := countWeekendDays(lastTransitionTime, transitionTime)
					if weekendDays > 0 && transitionDuration != 0 {
						transitionDuration -= time.Duration(weekendDays) * time.Hour * 24
					}

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

					// Do not include transition in calculations if outside parameter range
					if transitionTime.Before(startDate) || transitionTime.After(endDate) {
						if CLParameters.Debug {
							warn(" [IGNORED]\n")
						}
						continue
					}

					if containsStatus(wipStatus, item.ToString) {
						// Mapping var to calculate total WIP of the issue
						if wipTransitionTime == issueCreationTime {
							wipTransitionTime = transitionTime
						}
						// Mapping only WIP transitions to use for metrics later
						issueDetails.WIP += transitionDuration
						totalWipDuration += transitionDuration
						issueDetails.DurationByStatus[item.ToString] += transitionDuration
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
