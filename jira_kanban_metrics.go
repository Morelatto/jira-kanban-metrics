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
	"github.com/zchee/color" // TODO test colors on windows and terminator
	"math"
	"strings"
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

func printIssueDetailsByType(issueDetailsMapByType map[string][]IssueDetails) {
	const separator = " | "
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		title("\n>> %s\n", issueType)
		for _, issueDetails := range issueDetailsArray {
			startDate, endDate := formatBrDate(issueDetails.StartDate), formatBrDate(issueDetails.EndDate)
			toPrint := color.RedString(issueDetails.Name) + separator
			toPrint += color.WhiteString(issueDetails.Summary) + separator
			toPrint += color.YellowString("Start: %s", startDate) + separator
			toPrint += color.YellowString("End: %s", endDate) + separator
			toPrint += color.WhiteString("WIP: %s", durafmt.Parse(issueDetails.WIP))
			if issueDetails.EpicLink != "" {
				toPrint += separator
				toPrint += color.GreenString("Epic link: %v", issueDetails.EpicLink)
			}
			if len(issueDetails.Labels) > 0 {
				toPrint += separator
				toPrint += color.BlueString("Labels: %v", strings.Join(issueDetails.Labels, ", "))
			}
			if issueDetails.Sprint != "" {
				toPrint += separator
				toPrint += color.GreenString("Sprint: %v", issueDetails.Sprint)
			}
			if issueDetails.Resolved {
				toPrint += color.YellowString(" (Done)")
			}
			toPrint += "\n"
			_, _ = fmt.Fprintf(color.Output, toPrint)
		}
	}
}

func printAverageByStatus(totalDurationByStatusMap map[string]time.Duration, totalWipDuration time.Duration) {
	title("\n> Average by Status\n")
	for status, totalDuration := range totalDurationByStatusMap {
		statusPercent := float64(totalDuration*100) / float64(totalWipDuration)
		fmt.Printf("%v = %.2f%%\n", status, statusPercent)
	}
}

func printAverageByStatusType(totalDurationByStatusMap map[string]time.Duration, totalWipDuration time.Duration) {
	totalDurationByStatusTypeMap := make(map[string]time.Duration)
	for status, totalDuration := range totalDurationByStatusMap {
		if containsStatus(BoardCfg.OpenStatus, status) {
			totalDurationByStatusTypeMap["Open"] += totalDuration
		} else if containsStatus(BoardCfg.WipStatus, status) {
			totalDurationByStatusTypeMap["Wip"] += totalDuration
		} else if containsStatus(BoardCfg.IdleStatus, status) {
			totalDurationByStatusTypeMap["Idle"] += totalDuration
		} else if containsStatus(BoardCfg.DoneStatus, status) {
			totalDurationByStatusTypeMap["Done"] += totalDuration
		}
	}

	title("\n> Average by Status Type\n")
	for statusType, totalDuration := range totalDurationByStatusTypeMap {
		statusPercent := float64(totalDuration*100) / float64(totalWipDuration)
		fmt.Printf("%v = %.2f%% [%v] \n", statusType, statusPercent, totalDuration)
	}
}

func printWIP(totalWipDuration time.Duration, wipMonthly, weekDays int) {
	title("\n> WIP\n")
	fmt.Printf("Monthly: %v tasks\n", wipMonthly)
	totalWipDays := totalWipDuration.Hours() / 24
	fmt.Printf("Average: %.2f tasks\n", totalWipDays/float64(weekDays))
}

func printThroughput(throughputMonthly int, issueDetailsMapByType map[string][]IssueDetails) {
	title("\n> Throughput\n")
	fmt.Printf("Total: %v tasks delivered\n", throughputMonthly)
	fmt.Printf("By issue type:\n")
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		issueCount := len(issueDetailsArray)
		fmt.Printf("- %v: %v tasks (%v%%)\n", issueType, issueCount, (issueCount*100)/throughputMonthly)
	}
}

func printLeadTime(totalWipDuration time.Duration, throughputMonthly int, issueDetailsMapByType map[string][]IssueDetails) {
	var issueTypeLeadTimeMap = make(map[string]float64)
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		var wipDays []float64
		var totalWipByType time.Duration
		for _, issueDetails := range issueDetailsArray {
			totalWipByType += issueDetails.WIP
			wipDays = append(wipDays, float64(issueDetails.WIP))
		}
		totalWipAverageByIssueType := float64(totalWipByType) / float64(len(issueDetailsArray))
		issueTypeLeadTimeMap[issueType] = totalWipAverageByIssueType
	}

	title("\n> Lead time\n")
	totalWipDays := totalWipDuration.Hours() / 24
	fmt.Printf("Average: %v days\n", math.Round(totalWipDays/float64(throughputMonthly)))
	fmt.Printf("By issue type:\n")
	for issueType, leadTime := range issueTypeLeadTimeMap {
		fmt.Printf("- %v: %v days\n", issueType, math.Round(time.Duration(leadTime).Hours()/24))
	}
}
