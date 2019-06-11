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
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

func processCommandLineParameters() CLParameters {
	var parameters CLParameters

	if len(os.Args) < 5 {
		fmt.Printf("usage: %v <login> <startDate> <endDate> <jiraUrl> --debug\n", os.Args[0])
		fmt.Printf("example: %v user 01/31/2010 04/31/2010 http://jira.intranet/jira\n", os.Args[0])
		os.Exit(0)
	}

	parameters.Login = os.Args[1]
	parameters.StartDate = parseDate(os.Args[2])
	parameters.EndDate = parseDate(os.Args[3])
	parameters.JiraUrl = os.Args[4]
	parameters.Debug = false
	parameters.DebugVerbose = false

	if len(os.Args) == 6 {
		debugMethod := os.Args[5]
		if debugMethod == "--debug" {
			parameters.Debug = true
		} else if debugMethod == "--debug--verbose" {
			parameters.Debug = true
			parameters.DebugVerbose = true
		}
	}

	return parameters
}

func extractMonthlyThroughput(parameters CLParameters, auth Auth, boardCfg BoardCfg) int {
	troughputSearch := fmt.Sprintf("project = '%v' AND issuetype != Epic AND status CHANGED TO (%v) DURING('%v', '%v')",
		boardCfg.Project, formatColumns(boardCfg.DoneStatus), formatJiraDate(parameters.StartDate), formatJiraDate(parameters.EndDate))

	if parameters.Debug {
		fmt.Printf(TERM_COLOR_BLUE+"Troughput JQL: "+TERM_COLOR_WHITE+"%v\n\n", troughputSearch)
	}

	result := searchIssues(troughputSearch, parameters.JiraUrl, auth)
	return result.Total
}

func extractMetrics(parameters CLParameters, auth Auth, boardCfg BoardCfg) {
	throughtputMonthly := extractMonthlyThroughput(parameters, auth, boardCfg)

	startDate := formatJiraDate(parameters.StartDate)
	endDate := formatJiraDate(parameters.EndDate)

	wipSearch := fmt.Sprintf("project = '%v' AND  issuetype != Epic "+
		"AND (status CHANGED TO (%v) DURING('%v', '%v'))",
		boardCfg.Project,
		formatColumns(boardCfg.DoneStatus), startDate, endDate)

	if parameters.Debug {
		fmt.Printf(TERM_COLOR_BLUE+"WIP JQL: "+TERM_COLOR_WHITE+"%v\n\n", wipSearch)
	}

	result := searchIssues(wipSearch, parameters.JiraUrl, auth)
	wipMonthly := result.Total

	// Add one day to end date limit to include it in time comparisons
	parameters.EndDate = parameters.EndDate.Add(time.Hour * 24)

	var totalWipDays = 0                                  // Absolute number of WIP days of all issues during the specified period
	var issueTypeMap = make(map[string]int)               // Number of issues by type [key]
	var issueTypeLeadTimeMap = make(map[string]float64)   // Number of issues by type [key]
	var issueTypeConfidenceMap = make(map[string]float64) // Number of issues by type [key]

	var totalDurationByStatusMap = make(map[string]time.Duration)     // Duration by status type
	var totalDurationByStatusTypeMap = make(map[string]time.Duration) // Duration by status type
	var totalDuration time.Duration                                   // Total duration of all issues processed by the script (All status)
	var wipDuration time.Duration                                     // WIP duration of all issues (WIP/Idle)

	var issueDetailsMap = make(map[string]IssueDetails)
	var issueDetailsMapByType = make(map[string][]IssueDetails)

	// Transitions on the board: Issue -> Changelog -> Histories -> Items -> Field:Status
	for _, issue := range result.Issues {

		var issueDetails IssueDetails
		var resolved = false
		var epicLink string
		var sprint string

		var durationByStatusMap = make(map[string]int64)                  // Total duration [value] by status [key]
		var issueDurationByStatusMap = make(map[string]time.Duration)     // Total duration [value] by status [key]
		var issueDurationByStatusTypeMap = make(map[string]time.Duration) // Total duration [value] by status [key]

		var lastToStatus string
		var transitionToWipDate time.Time

		var issueCreatedDate = parseJiraTime(issue.Fields.Created)
		var lastFromStatusCreationDate = issueCreatedDate

		if parameters.DebugVerbose {
			fmt.Printf(TERM_COLOR_YELLOW+"\n%v\n", issue.Key)
		}

		for _, history := range issue.Changelog.Histories {
			for _, item := range history.Items {
				// Ignore transitions to the same status
				if item.Fromstring == item.Tostring {
					continue
				}

				if item.Field == "status" {
					// Timestamp when the transition happened
					statusChangeTime := parseJiraTime(history.Created)

					wipStatus := append(boardCfg.WipStatus, boardCfg.IdleStatus...)
					// Mapping var to calculate total WIP of the issue
					if transitionToWipDate.IsZero() && containsStatus(wipStatus, item.Tostring) {
						transitionToWipDate = statusChangeTime
					}

					// Calculating status transition duration
					statusChangeDuration := statusChangeTime.Sub(lastFromStatusCreationDate)
					weekendDaysBetweenDates := countWeekendDays(lastFromStatusCreationDate, statusChangeTime)
					if weekendDaysBetweenDates > 0 {
						updatedTotalSeconds := statusChangeDuration.Seconds() - float64(60*60*24*weekendDaysBetweenDates)
						statusChangeDuration = time.Duration(updatedTotalSeconds) * time.Second
						if parameters.DebugVerbose {
							fmt.Printf(TERM_COLOR_RED+"Removing weekend days [%v] from Status [%v] \n"+TERM_COLOR_YELLOW, weekendDaysBetweenDates, item.Fromstring)
						}
					}

					if parameters.DebugVerbose {
						printIssueTransition(statusChangeTime, lastFromStatusCreationDate, statusChangeDuration, item.Fromstring, item.Tostring)
					}

					// Group total minutes by status, considering this status transition
					durationByStatusMap[item.Fromstring] = durationByStatusMap[item.Fromstring] + int64(statusChangeDuration.Minutes())
					issueDurationByStatusMap[item.Fromstring] = issueDurationByStatusMap[item.Fromstring] + statusChangeDuration

					// Update vars for next iteration
					lastToStatus = item.Tostring
					lastFromStatusCreationDate = statusChangeTime

				} else if item.Field == "Epic Link" {
					epicLink = item.Tostring
				} else if item.Field == "Sprint" {
					sprint = item.Tostring
				}
			}
		}

		// Calculate the duration of the last transition, if it's not done
		if lastFromStatusCreationDate.Before(parameters.EndDate) && !containsStatus(boardCfg.DoneStatus, lastToStatus) {
			statusChangeDuration := parameters.EndDate.Sub(lastFromStatusCreationDate)

			// Group total minutes by status, considering this status transition
			durationByStatusMap[lastToStatus] = durationByStatusMap[lastToStatus] + int64(statusChangeDuration.Minutes())
			issueDurationByStatusMap[lastToStatus] = issueDurationByStatusMap[lastToStatus] + statusChangeDuration

			// Print debug
			if parameters.Debug {
				fmt.Printf(TERM_COLOR_RED+"Status current in development, considering endDate [%v] \n"+TERM_COLOR_WHITE, formatBrDateWithTime(parameters.EndDate))
			}

			if parameters.DebugVerbose {
				printIssueTransition(parameters.EndDate, lastFromStatusCreationDate, statusChangeDuration, lastToStatus, "None")
			}
		}

		// Calculate the duration of all status
		if parameters.Debug {
			fmt.Printf(TERM_COLOR_BLUE+"\n%v\n"+TERM_COLOR_WHITE, issue.Key)
		}

		var issueTotalDuration time.Duration
		var statusType string

		for k, v := range issueDurationByStatusMap {
			if containsStatus(boardCfg.OpenStatus, k) {
				statusType = "Open"
			} else if containsStatus(boardCfg.WipStatus, k) {
				statusType = "Wip"
			} else if containsStatus(boardCfg.IdleStatus, k) {
				statusType = "Idle"
			} else if containsStatus(boardCfg.DoneStatus, k) {
				statusType = "Done"
			} else {
				fmt.Printf("Status %v not mapped in board.cfg, please update it.\n", k)
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

			if parameters.Debug {
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
		if containsStatus(boardCfg.DoneStatus, lastToStatus) {
			resolved = true

			// Double check if the wip is being calculated correct, it's not used for anything else
			issueTotalWip := subtractDatesRemovingWeekends(transitionToWipDate, lastFromStatusCreationDate)
			wipDiffBetweenCalcMethods := issueDurationTotalWip - issueTotalWip
			if parameters.Debug && (wipDiffBetweenCalcMethods.Hours() > 1 || wipDiffBetweenCalcMethods.Hours() < -1) {
				fmt.Printf(TERM_COLOR_RED + "Issue has some strange status transition. Please check it!!! \n" + TERM_COLOR_WHITE)
			}
		}

		if resolved {
			issueTypeMap[issue.Fields.Issuetype.Name]++
		}

		// Calculating percentage by status type configured in board.cfg
		for k, v := range issueDurationByStatusTypeMap {
			statusPercent := float64(v*100) / float64(issueTotalDuration)

			// Adding it to the total count
			totalDurationByStatusTypeMap[k] += v

			// Print details if in debug mode
			if parameters.Debug {
				fmt.Printf("%v = %.2f%% [%v] \n", k, statusPercent, v)
			}
		}

		// Print status transition details by issue if in debug Mode
		if parameters.DebugVerbose {
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
		issueDetails.IssueType = issue.Fields.Issuetype.Name
		issueDetails.Resolved = resolved
		issueDetails.Labels = issue.Fields.Labels

		issueDetailsMap[issueDetails.Name] = issueDetails

		issueArray := issueDetailsMapByType[issueDetails.IssueType]
		issueArray = append(issueArray, issueDetails)
		issueDetailsMapByType[issueDetails.IssueType] = issueArray
	}

	lastType := ""
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		if lastType != issueType {
			lastType = issueType
			fmt.Printf("\n>> %v\n", issueType)
		}

		var wipDays []float64
		totalWipDaysByIssueType := 0
		for _, issueDetails := range issueDetailsArray {
			fmt.Printf(TERM_COLOR_BLUE+"%v | %v | Start: %v| End: %v | WIP days: %v | ", issueDetails.Name, issueDetails.Summary,
				formatBrDate(issueDetails.StartDate), formatBrDate(issueDetails.EndDate), issueDetails.WIP)

			if issueDetails.EpicLink != "" {
				fmt.Printf(" Epic link: %v |", issueDetails.EpicLink)
			}

			if len(issueDetails.Labels) > 0 {
				fmt.Printf(" Labels: %v |", strings.Join(issueDetails.Labels, ", "))
			}

			if issueDetails.Sprint != "" {
				fmt.Printf(" Sprint: %v |", issueDetails.Sprint)
			}

			if issueDetails.Resolved {
				fmt.Printf(TERM_COLOR_YELLOW + " (Done)" + TERM_COLOR_WHITE + "\n")
			} else {
				fmt.Print(TERM_COLOR_WHITE + "\n")
			}

			totalWipDaysByIssueType += issueDetails.WIP
			wipDays = append(wipDays, float64(issueDetails.WIP))
		}

		totalWipAverageByIssueType := float64(totalWipDaysByIssueType) / float64(len(issueDetailsArray))
		issueTypeLeadTimeMap[issueType] = totalWipAverageByIssueType
		issueTypeConfidenceMap[issueType] = confidence90(wipDays)

		fmt.Printf("Average lead time: %v\n", math.Round(totalWipAverageByIssueType))
		fmt.Printf("Median lead time: %v\n", median(wipDays))
		fmt.Printf("Confidence lead time: %v\n", confidence90(wipDays))
	}

	fmt.Printf("\n> Average by Status\n")
	for k, v := range totalDurationByStatusMap {
		statusPercent := float64(v*100) / float64(wipDuration)
		fmt.Printf("%v = %.2f%% [%v] \n", k, statusPercent, v)
	}

	fmt.Printf("\n> Average by Status Type\n")
	for k, v := range totalDurationByStatusTypeMap {
		statusPercent := float64(v*100) / float64(totalDuration)
		fmt.Printf("%v = %.2f%% [%v] \n", k, statusPercent, v)
	}

	weekDays := countWeekDays(parameters.StartDate, parameters.EndDate)

	fmt.Printf("\n> WIP\n")
	fmt.Printf("Monthly: %v tasks\n", wipMonthly)
	if totalWipDays > 0 {
		fmt.Printf("Average: %.2f tasks\n", float64(totalWipDays)/float64(weekDays))
	}

	fmt.Printf("\n> Throughput\n")
	fmt.Printf("Total: %v tasks delivered\n", throughtputMonthly)
	fmt.Printf("By issue type:\n")
	for key, value := range issueTypeMap {
		fmt.Printf("- %v: %v tasks (%v%%)\n", key, value, (value*100)/throughtputMonthly)
	}

	fmt.Printf("\n> Lead time\n")
	fmt.Printf("Total: %v days\n", math.Round(float64(totalWipDays)/float64(throughtputMonthly)))
	fmt.Printf("By issue type:\n")
	for issueType, leadTime := range issueTypeLeadTimeMap {
		fmt.Printf("- %v: %v days - 90%% < %v days \n", issueType, math.Round(leadTime), math.Round(issueTypeConfidenceMap[issueType]))
	}

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

func main() {
	var parameters = processCommandLineParameters()

	var auth = authenticate(parameters.Login, parameters.JiraUrl)

	boardCfg := loadBoardCfg()

	fmt.Printf("Extracting Kanban metrics from project %v, %v to %v\n\n",
		boardCfg.Project, formatJiraDate(parameters.StartDate), formatJiraDate(parameters.EndDate))

	extractMetrics(parameters, auth, boardCfg)
}

func fmtDuration(d time.Duration) string {
	days := int(d.Hours()) / int(24)
	hours := int(d.Hours() - float64(days*int(24)))
	minutes := int(d.Minutes() - float64((days*24*60)+(hours*int(60))))
	return fmt.Sprintf("%vd %vh %vm", days, hours, minutes)
}

func printIssueTransition(statusChangeTime time.Time, statusChangeTimeStart time.Time, statusChangeDuration time.Duration, statusFrom string, statusTo string) {
	fmt.Printf("%v - %v\n", formatBrDateWithTime(statusChangeTimeStart), formatBrDateWithTime(statusChangeTime))
	fmt.Printf("[%v] (%v) -> [%v]\n\n", statusFrom, fmtDuration(statusChangeDuration), statusTo)
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
