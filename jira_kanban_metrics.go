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
	"github.com/zchee/color" // TODO test colors on windows and terminator
	"math"
	"sort"
	"strings"
	"time"
)

var usage = `Jira Kanban Metrics.

Usage: 
  jira-kanban-metrics <startDate> <endDate> [--debug]
  jira-kanban-metrics -h | --help
  jira-kanban-metrics --version

Arguments:
  startDate  Start date in dd/mm/yyyy format.
  endDate    End date in dd/mm/yyyy format.

Options:
  -h --help  Show this screen.
  --version  Show version.
  --debug    Debug mode [default: false].
`

func main() {
	arguments, _ := docopt.ParseArgs(usage, nil, "1.0")
	arguments.Bind(&CLParameters)

	loadBoardCfg()
	authJiraClient()

	jqlSearch := getIssuesBetweenInProjectWithStatus(CLParameters.StartDate, CLParameters.EndDate, BoardCfg.Project, BoardCfg.DoneStatus)
	extractMetrics(searchIssues(jqlSearch))
}

func extractMetrics(issues []jira.Issue) {
	fmt.Printf("Extracting Kanban metrics from project %v, %v to %v\n",
		BoardCfg.Project, CLParameters.StartDate, CLParameters.EndDate)

	throughputMonthly := len(issues)
	wipMonthly := len(issues)
	wipStatus := append(BoardCfg.WipStatus, BoardCfg.IdleStatus...)

	// Add one day to end date limit to include it in time comparisons
	endDate := parseDate(CLParameters.EndDate).Add(time.Hour * 24)

	var totalWipDays = 0                    // Absolute number of WIP days of all issues during the specified period
	var issueTypeMap = make(map[string]int) // Number of issues by type [key]
	var issueTypeLeadTimeMap = make(map[string]float64)
	var issueTypeConfidenceMap = make(map[string]float64)

	var totalDurationByStatusMap = make(map[string]time.Duration)     // Duration by status name
	var totalDurationByStatusTypeMap = make(map[string]time.Duration) // Duration by status type (wip, idle)
	var totalDuration time.Duration                                   // Total duration of all issues processed by the script (All status)
	var wipDuration time.Duration                                     // WIP duration of all issues (WIP/Idle)

	var issueDetailsMap = make(map[string]IssueDetails)
	var issueDetailsMapByType = make(map[string][]IssueDetails)

	var notMappedStatus = make(map[string]int)

	// Transitions on the board: Issue -> Changelog -> Histories -> Items -> Field:Status
	for _, issue := range issues {
		var issueDetails IssueDetails
		var resolved = false
		var epicLink string
		var sprint string

		var issueDurationByStatusMap = make(map[string]time.Duration)     // Total issue duration by status name
		var issueDurationByStatusTypeMap = make(map[string]time.Duration) // Total issue duration by status type

		var lastToStatus string
		var transitionToWipDate time.Time

		var issueCreatedDate = time.Time(issue.Fields.Created)
		var lastFromStatusCreationDate = issueCreatedDate

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
					statusChangeTime := parseJiraTime(history.Created)

					// Mapping var to calculate total WIP of the issue
					if transitionToWipDate.IsZero() && containsStatus(wipStatus, item.ToString) {
						transitionToWipDate = statusChangeTime
					}

					// Calculating status transition duration
					statusChangeDuration := calculateStatusChangeDuration(statusChangeTime, lastFromStatusCreationDate, item.FromString, item.ToString)

					// Group total minutes by status, considering this status transition
					issueDurationByStatusMap[item.FromString] = issueDurationByStatusMap[item.FromString] + statusChangeDuration

					// Update vars for next iteration
					lastToStatus = item.ToString
					lastFromStatusCreationDate = statusChangeTime
				} else if item.Field == "Epic Link" {
					epicLink = item.ToString
				} else if item.Field == "Sprint" {
					sprint = item.ToString
				}
			}
		}

		// FIXME considers endDate of opened issue as today, is this right?
		// Calculate the duration of the last transition, if it's not done (current in dev)
		if lastFromStatusCreationDate.Before(endDate) && !containsStatus(BoardCfg.DoneStatus, lastToStatus) {
			statusChangeDuration := endDate.Sub(lastFromStatusCreationDate)

			// Group total minutes by status, considering this status transition
			issueDurationByStatusMap[lastToStatus] = issueDurationByStatusMap[lastToStatus] + statusChangeDuration

			if CLParameters.Debug {
				warn("Status current in development, considering endDate [%s]\n", formatBrDateWithTime(endDate))
				printIssueTransition(endDate, lastFromStatusCreationDate, statusChangeDuration, lastToStatus, "None")
			}
		}

		// Calculate the duration of all status
		var issueTotalDuration time.Duration
		var statusType string

		for k, v := range issueDurationByStatusMap {
			if containsStatus(BoardCfg.OpenStatus, k) {
				statusType = "Open"
			} else if containsStatus(BoardCfg.WipStatus, k) {
				statusType = "Wip"
			} else if containsStatus(BoardCfg.IdleStatus, k) {
				statusType = "Idle"
			} else if containsStatus(BoardCfg.DoneStatus, k) {
				statusType = "Done"
			} else {
				notMappedStatus[k]++
				continue
			}

			// Adding it to the total count only if in WIP/Idle
			if statusType == "Wip" || statusType == "Idle" {
				wipDuration += v
				issueTotalDuration += v
				totalDurationByStatusMap[k] += v

				issueDurationByStatusTypeMap[statusType] = issueDurationByStatusTypeMap[statusType] + v
				totalDuration += v
			}

			if CLParameters.Debug {
				fmt.Printf("Status [%v] time in [%v] \n", k, v)
			}
		}

		// Calculating WIP days
		issueDurationTotalWip := issueDurationByStatusTypeMap["Wip"] + issueDurationByStatusTypeMap["Idle"]
		issueWipDays := int(issueDurationTotalWip.Hours()) / 24
		if issueWipDays == 0 {
			issueWipDays = 1
		}
		totalWipDays += issueWipDays

		// Verify if the last transition is to a resolved status
		if containsStatus(BoardCfg.DoneStatus, lastToStatus) {
			resolved = true

			// Double check if the wip is being calculated correct, it's not used for anything else
			issueTotalWip := subtractDatesRemovingWeekends(transitionToWipDate, lastFromStatusCreationDate)
			wipDiffBetweenCalcMethods := issueDurationTotalWip - issueTotalWip
			if CLParameters.Debug && (wipDiffBetweenCalcMethods.Hours() > 1 || wipDiffBetweenCalcMethods.Hours() < -1) {
				color.Red("Issue has some strange status transition. Please check it!!!")
			}
		}

		if resolved {
			issueTypeMap[issue.Fields.Type.Name]++
		}

		// Calculating percentage by status type configured in board.cfg
		for k, v := range issueDurationByStatusTypeMap {
			statusPercent := float64(v*100) / float64(issueTotalDuration)

			// Adding it to the total count
			totalDurationByStatusTypeMap[k] += v

			// Print details if in debug mode
			if CLParameters.Debug {
				info("%s = %.2f%% [%s] \n", k, statusPercent, v)
			}
		}

		// Print status transition details by issue if in debug Mode
		if CLParameters.Debug {
			fmt.Print("\n>Status Transition Details\n")
			for k, v := range issueDurationByStatusMap {
				statusPercent := float64(v*100) / float64(issueTotalDuration)
				fmt.Printf("%v = %.2f%% [%v] \n", k, statusPercent, v)
			}
		}

		issueDetails.Name = issue.Key
		issueDetails.Summary = issue.Fields.Summary
		issueDetails.StartDate = transitionToWipDate
		issueDetails.EndDate = lastFromStatusCreationDate
		issueDetails.WIP = issueWipDays
		issueDetails.EpicLink = epicLink
		issueDetails.Sprint = sprint
		issueDetails.IssueType = issue.Fields.Type.Name
		issueDetails.Resolved = resolved
		issueDetails.Labels = issue.Fields.Labels

		issueDetailsMap[issueDetails.Name] = issueDetails
		issueDetailsMapByType[issueDetails.IssueType] = append(issueDetailsMapByType[issueDetails.IssueType], issueDetails)
	}

	if CLParameters.Debug {
		fmt.Println("\nThe following status were found but not mapped in board.cfg:")
		for status := range notMappedStatus {
			fmt.Println(status)
		}
	}

	printIssueDetailsByType(issueDetailsMapByType, issueTypeLeadTimeMap, issueTypeConfidenceMap)
	printAverageByStatus(totalDurationByStatusMap, wipDuration)
	printAverageByStatusType(totalDurationByStatusTypeMap, totalDuration)
	printWIP(wipMonthly, totalWipDays, parseDate(CLParameters.StartDate), endDate)
	printThroughput(throughputMonthly, issueTypeMap)
	printLeadTime(totalWipDays, throughputMonthly, issueTypeLeadTimeMap, issueTypeConfidenceMap)
	printDataForScaterplot(issueTypeMap, issueDetailsMap, issueTypeConfidenceMap)
}

func printIssueDetailsByType(issueDetailsMapByType map[string][]IssueDetails, issueTypeLeadTimeMap map[string]float64, issueTypeConfidenceMap map[string]float64) {
	lastType := ""
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		if lastType != issueType {
			lastType = issueType
			title("\n>> %s\n", issueType)
		}

		var wipDays []float64
		totalWipDaysByIssueType := 0
		for _, issueDetails := range issueDetailsArray {
			startDate, endDate := formatBrDate(issueDetails.StartDate), formatBrDate(issueDetails.EndDate)
			toPrint := color.BlueString("%s | %s | Start: %s| End: %s | WIP days: %d", issueDetails.Name, issueDetails.Name, startDate, endDate, issueDetails.WIP)

			if issueDetails.EpicLink != "" {
				toPrint += color.BlueString(" | Epic link: %v", issueDetails.EpicLink)
			}

			if len(issueDetails.Labels) > 0 {
				toPrint += color.CyanString(" | Labels: %v", strings.Join(issueDetails.Labels, ", "))
			}

			if issueDetails.Sprint != "" {
				toPrint += color.GreenString(" | Sprint: %v", issueDetails.Sprint)
			}

			if issueDetails.Resolved {
				toPrint += color.YellowString(" (Done)")
			}
			toPrint += "\n"

			fmt.Fprintf(color.Output, toPrint)
			totalWipDaysByIssueType += issueDetails.WIP
			wipDays = append(wipDays, float64(issueDetails.WIP))
		}

		totalWipAverageByIssueType := float64(totalWipDaysByIssueType) / float64(len(issueDetailsArray))
		issueTypeLeadTimeMap[issueType] = totalWipAverageByIssueType
		issueTypeConfidenceMap[issueType] = confidence90(wipDays)

		if CLParameters.Debug {
			fmt.Printf("Average lead time: %v\n", math.Round(totalWipAverageByIssueType))
			fmt.Printf("Median lead time: %v\n", median(wipDays))
			fmt.Printf("Confidence lead time: %v\n", confidence90(wipDays))
		}
	}
}

func printAverageByStatus(totalDurationByStatusMap map[string]time.Duration, wipDuration time.Duration) {
	fmt.Printf("\n> Average by Status\n")
	for k, v := range totalDurationByStatusMap {
		statusPercent := float64(v*100) / float64(wipDuration)
		fmt.Printf("%v = %.2f%% [%v] \n", k, statusPercent, v)
	}
}

func printAverageByStatusType(totalDurationByStatusTypeMap map[string]time.Duration, totalDuration time.Duration) {
	fmt.Printf("\n> Average by Status Type\n")
	for k, v := range totalDurationByStatusTypeMap {
		statusPercent := float64(v*100) / float64(totalDuration)
		fmt.Printf("%v = %.2f%% [%v] \n", k, statusPercent, v)
	}
}

func printWIP(wipMonthly int, totalWipDays int, startDate, endDate time.Time) {
	weekDays := countWeekDays(startDate, endDate)
	fmt.Printf("\n> WIP\n")
	fmt.Printf("Monthly: %v tasks\n", wipMonthly)
	if totalWipDays > 0 {
		fmt.Printf("Average: %.2f tasks\n", float64(totalWipDays)/float64(weekDays))
	}
}

func printThroughput(throughtputMonthly int, issueTypeMap map[string]int) {
	fmt.Printf("\n> Throughput\n")
	fmt.Printf("Total: %v tasks delivered\n", throughtputMonthly)
	fmt.Printf("By issue type:\n")
	for key, value := range issueTypeMap {
		fmt.Printf("- %v: %v tasks (%v%%)\n", key, value, (value*100)/throughtputMonthly)
	}
}

func printLeadTime(totalWipDays int, throughtputMonthly int, issueTypeLeadTimeMap map[string]float64, issueTypeConfidenceMap map[string]float64) {
	fmt.Printf("\n> Lead time\n")
	fmt.Printf("Total: %v days\n", math.Round(float64(totalWipDays)/float64(throughtputMonthly)))
	fmt.Printf("By issue type:\n")
	for issueType, leadTime := range issueTypeLeadTimeMap {
		fmt.Printf("- %v: %v days - 90%% < %v days \n", issueType, math.Round(leadTime), math.Round(issueTypeConfidenceMap[issueType]))
	}
}

func printDataForScaterplot(issueTypeMap map[string]int, issueDetailsMap map[string]IssueDetails, issueTypeConfidenceMap map[string]float64) {
	fmt.Printf("\n> Data for scaterplot\n")
	for issueType := range issueTypeMap {
		fmt.Printf(">> %v\n", issueType)
		for _, v := range issueDetailsMap {
			if v.IssueType == issueType {

				var outlier = ""
				if v.WIP > int(math.Round(issueTypeConfidenceMap[issueType])) {
					outlier = "Outlier"
				}

				// fmt.Printf("%v;%v;%v;%v;%v;\n", v.Name, formatBrDate(v.StartDate), formatBrDate(v.EndDate), v.WIP, v.EpicLink)
				fmt.Printf("%v;%v;%v;%v;%v\n", formatBrDate(v.EndDate), v.WIP, v.Name, v.EpicLink, outlier)
			}
		}

		fmt.Printf("\n")
	}
}

func calculateStatusChangeDuration(statusChangeTime, lastFromStatusCreationDate time.Time, statusFrom, statusTo string) time.Duration {
	statusChangeDuration := statusChangeTime.Sub(lastFromStatusCreationDate)
	weekendDaysBetweenDates := countWeekendDays(lastFromStatusCreationDate, statusChangeTime)
	if weekendDaysBetweenDates > 0 {
		updatedTotalSeconds := statusChangeDuration.Seconds() - float64(60*60*24*weekendDaysBetweenDates)
		statusChangeDuration = time.Duration(updatedTotalSeconds) * time.Second
		//if debugVerbose {
		//	fmt.Printf(TERM_COLOR_RED+"Removing weekend days [%v] from Status [%v] \n"+TERM_COLOR_YELLOW, weekendDaysBetweenDates, statusFrom)
		//}
	}

	if CLParameters.Debug {
		printIssueTransition(statusChangeTime, lastFromStatusCreationDate, statusChangeDuration, statusFrom, statusTo)
	}
	return statusChangeDuration
}

func fmtDuration(d time.Duration) string {
	days := int(d.Hours()) / int(24)
	hours := int(d.Hours() - float64(days*int(24)))
	minutes := int(d.Minutes() - float64((days*24*60)+(hours*int(60))))
	return fmt.Sprintf("%vd %vh %vm", days, hours, minutes)
}

func printIssueTransition(statusChangeTime time.Time, statusChangeTimeStart time.Time, statusChangeDuration time.Duration, statusFrom string, statusTo string) {
	color.Set(color.FgYellow, color.Bold)
	defer color.Unset()
	fmt.Printf("%v - %v", formatBrDateWithTime(statusChangeTimeStart), formatBrDateWithTime(statusChangeTime))
	fmt.Printf("\n[%v] (%v) -> [%v]\n", statusFrom, fmtDuration(statusChangeDuration), statusTo)
}

func median(numbers []float64) float64 {
	sort.Float64s(numbers)
	middle := len(numbers) / 2
	result := numbers[middle]
	if len(numbers)%2 == 0 {
		result = (result + numbers[middle-1]) / 2
	}
	return result
}

func average(values []float64) float64 {
	total := float64(0)
	for _, value := range values {
		total += float64(value)
	}
	median := total / float64(len(values))
	return median
}

func variation(values []float64, median float64) float64 {
	total := float64(0)
	for _, value := range values {
		diff := value - median
		total += math.Pow(diff, 2)

	}
	return total / float64(len(values)-1)
}

func confidence(median float64, standarDeviation float64) float64 {
	return median + (1.644854 * standarDeviation)
}

func confidence90(values []float64) float64 {
	average := average(values)
	variation := variation(values, average)
	standardDeviation := math.Sqrt(variation)
	confidence := confidence(average, standardDeviation)
	return confidence
}
