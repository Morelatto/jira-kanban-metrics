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
	"log"
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

const version = "1.1"

func main() {
	arguments, _ := docopt.ParseArgs(usage, nil, version)
	err := arguments.Bind(&CLParameters)
	if err != nil {
		log.Fatalf("Failed to parse command line arguments: %v", err)
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

	doneIssuesByTypeMap, notMapped := extractMetrics(doneIssues, wipStatus)
	notDoneIssuesByTypeMap, notMapped2 := extractMetrics(notDoneIssues, wipStatus)
	notMapped = append(notMapped, notMapped2...)

	if len(notMapped) > 0 {
		warn("\nThe following status were found but not mapped in board.cfg:\n")
		for _, status := range notMapped {
			fmt.Println(status)
		}
	}

	printIssueDetailsByType(mergeMaps(doneIssuesByTypeMap, notDoneIssuesByTypeMap))
	printAverageByStatus(doneIssuesByTypeMap)
	printAverageByStatusType(doneIssuesByTypeMap)
	printWIP(doneIssuesByTypeMap, countWeekDays(startDate, endDate))
	printThroughput(doneIssuesByTypeMap)
	printLeadTime(doneIssuesByTypeMap)
	// TODO print scaterplot
}

type TransitionDetails struct {
	Timestamp          time.Time
	StatusFrom         string
	StatusTo           string
	PreviousTransition *TransitionDetails
}

func (t *TransitionDetails) getTotalDuration() time.Duration {
	return getTransitionDuration(t.PreviousTransition.Timestamp, t.Timestamp)
}

func (t *TransitionDetails) PrintFrom() {
	info("%s %s\n", formatBrDateWithTime(t.PreviousTransition.Timestamp), t.StatusFrom)
}

func (t *TransitionDetails) PrintTo() {
	info("%s %s", formatBrDateWithTime(t.Timestamp), t.StatusTo)
	if t.PreviousTransition != nil {
		warn(" [%s]", durafmt.Parse(t.getTotalDuration()))
	}
	fmt.Println()
}

func extractMetrics(issues []jira.Issue, wipStatus []string) (map[string][]IssueDetails, []string) {
	var issueDetailsMapByType = make(map[string][]IssueDetails)
	var notMappedStatus []string

	// Transitions on the board: Issue -> Changelog -> Histories -> Items -> Field:Status
	for _, issue := range issues {
		issueDetails := New(issue)

		issueCreationTime := time.Time(issue.Fields.Created)
		wipTransitionTime := issueCreationTime
		lastTransitionTime := issueCreationTime

		Debug(issue.Key)
		var previousTransition = &TransitionDetails{
			Timestamp: time.Time(issue.Fields.Created),
			StatusTo:  "Open",
		}
		previousTransition.PrintTo()
		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				if item.Field == "status" {
					transitionTime, _ := history.CreatedTime()
					transitionDuration := getTransitionDuration(lastTransitionTime, transitionTime)

					var t = TransitionDetails{
						Timestamp:          transitionTime,
						StatusFrom:         item.FromString,
						StatusTo:           item.ToString,
						PreviousTransition: previousTransition,
					}
					if CLParameters.Debug {
						t.PrintTo()
					}

					// Check if status is not mapped on cfg to warn the user
					if !containsStatus(notMappedStatus, item.ToString) && statusIsNotMapped(item.ToString) {
						notMappedStatus = append(notMappedStatus, item.ToString)
					}

					// Update vars for next iteration
					previousTransition = &t
					lastTransitionTime = transitionTime
					issueDetails.LastStatus = item.ToString

					if containsStatus(wipStatus, item.FromString) || containsStatus(BoardCfg.DoneStatus, item.ToString) {
						// Mapping var to calculate total WIP of the issue
						if wipTransitionTime == issueCreationTime {
							wipTransitionTime = transitionTime
						}
						// Mapping only WIP transitions to use for metrics later
						issueDetails.WIPIdle += transitionDuration
						issueDetails.DurationByStatus[item.FromString] += transitionDuration

						if containsStatus(BoardCfg.WipStatus, item.ToString) || containsStatus(BoardCfg.DoneStatus, item.ToString) {
							issueDetails.WIP += transitionDuration
						}
					}

					if containsStatus(BoardCfg.DoneStatus, item.ToString) {
						issueDetails.Resolved = true
						issueDetails.ResolvedDate = transitionTime
					}
				} else if item.Field == "Epic Link" {
					issueDetails.EpicLink = item.ToString
				} else if item.Field == "Sprint" {
					issueDetails.Sprint = item.ToString
				}
			}
		}
		issueDetails.TransitionDetails = previousTransition

		// Consider WIP date until now if last status on WIP
		if !issueDetails.Resolved && containsStatus(wipStatus, issueDetails.LastStatus) {
			transitionDuration := getTransitionDuration(lastTransitionTime, time.Now())
			issueDetails.WIPIdle += transitionDuration
			issueDetails.DurationByStatus[issueDetails.LastStatus] += transitionDuration
			if containsStatus(BoardCfg.WipStatus, issueDetails.LastStatus) {
				issueDetails.WIP += transitionDuration
			}
		}
		issueDetails.ToWipDate = wipTransitionTime
		issueDetailsMapByType[issueDetails.IssueType] = append(issueDetailsMapByType[issueDetails.IssueType], issueDetails)
	}

	return issueDetailsMapByType, notMappedStatus
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
