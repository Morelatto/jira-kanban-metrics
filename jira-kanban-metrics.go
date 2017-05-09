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
	"os"
	"fmt"
	"time"
)

const TERM_COLOR_BLUE string = "\x1b[94;1m"
const TERM_COLOR_YELLOW string = "\x1b[93;1m"
const TERM_COLOR_RED string = "\x1b[91;1m"
const TERM_COLOR_WHITE string = "\x1b[0m"

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

	if len(os.Args) == 6 {
		if os.Args[5] == "--debug" {
			parameters.Debug = true
		}
	}

	return parameters
}

func extractMonthlyThroughput(parameters CLParameters, auth Auth, boardCfg BoardCfg) int {
	troughputSearch := fmt.Sprintf("project = '%v' AND issuetype != Epic AND status CHANGED TO %v DURING('%v', '%v')", 
								   boardCfg.Project, formatColumns(boardCfg.DoneStatus), formatJiraDate(parameters.StartDate), formatJiraDate(parameters.EndDate))

	if parameters.Debug {
		fmt.Printf(TERM_COLOR_BLUE + "Troughput JQL: " + TERM_COLOR_WHITE + "%v\n\n", troughputSearch)
	}

	result := searchIssues(troughputSearch, parameters.JiraUrl, auth)
	return result.Total
}

func extractMetrics(parameters CLParameters, auth Auth, boardCfg BoardCfg) {
	throughtputMonthly := extractMonthlyThroughput(parameters, auth, boardCfg)

	startDate := formatJiraDate(parameters.StartDate)
	endDate := formatJiraDate(parameters.EndDate)

	wipSearch := fmt.Sprintf("project = '%v' AND issuetype != Epic AND (status WAS IN (%v) " + 
							 "DURING('%v', '%v') or status CHANGED TO %v DURING('%v', '%v'))", 
							 boardCfg.Project, formatColumns(boardCfg.WipStatus), startDate, endDate, formatColumns(boardCfg.DoneStatus), startDate, endDate)

	if parameters.Debug {
		fmt.Printf(TERM_COLOR_BLUE + "WIP JQL: " + TERM_COLOR_WHITE + "%v\n\n", wipSearch)
	}

	result := searchIssues(wipSearch, parameters.JiraUrl, auth)
	wipMonthly := result.Total

	var wipDays int = 0
	var idleDays int = 0
	var issueTypeMap map[string]int = make(map[string]int)

	// Transitions on the board: Issue -> Changelog -> Histories -> Items -> Field:Status
	for _, issue := range result.Issues {

		var wipTransitionDate time.Time
		var doneTransitionDate time.Time = parameters.EndDate

		var idleStart time.Time
		var idleEnd time.Time
		var isIdle bool = false
		var issueDaysInIdle int = 0

		var resolved bool = false

		for _, history := range issue.Changelog.Histories {

			for _, item := range history.Items {

				if item.Field == "status" {

					// Date when the transition happened
					statusChangeTime := stripHours(parseJiraTime(history.Created))

					// Transition from OPEN to WIP
					// Consider only the first transition to WIP, a task should not go back on a kanban board
					if (containsStatus(boardCfg.OpenStatus, item.Fromstring) && containsStatus(boardCfg.WipStatus, item.Tostring) && wipTransitionDate.IsZero()) {
						wipTransitionDate = statusChangeTime
						resolved = false

						if wipTransitionDate.Before(parameters.StartDate) {
							wipTransitionDate = parameters.StartDate
						}
					}

					// Transition from WIP to DONE
					if (containsStatus(boardCfg.WipStatus, item.Fromstring) && containsStatus(boardCfg.DoneStatus, item.Tostring)) {
						doneTransitionDate = parameters.EndDate

						// If the transition happened during the period, the task is resolved
						if statusChangeTime.Before(parameters.EndDate) || statusChangeTime.Equal(parameters.EndDate) {
							doneTransitionDate = statusChangeTime
							resolved = true
						}
					}

					// Transition from OPEN to DONE
					if (containsStatus(boardCfg.OpenStatus, item.Fromstring) && containsStatus(boardCfg.DoneStatus, item.Tostring)) {
						wipTransitionDate = statusChangeTime
						doneTransitionDate = statusChangeTime
						resolved = true

						if wipTransitionDate.Before(parameters.StartDate) {
							wipTransitionDate = parameters.StartDate
						}
						if doneTransitionDate.After(parameters.EndDate) {
							doneTransitionDate = parameters.EndDate
						}
					}

					// Transition to IDLE
					if containsStatus(boardCfg.IdleStatus, item.Tostring) {
						idleStart = statusChangeTime
						isIdle = true

					// Transition from IDLE
					} else if containsStatus(boardCfg.IdleStatus, item.Fromstring) {
						idleEnd = statusChangeTime
						isIdle = false
						issueDaysInIdle += countWeekDays(idleStart, idleEnd)
					}

					// Log debug the transition
					if parameters.Debug {
						fmt.Printf("%v -> %v (%v) ", item.Fromstring, item.Tostring, formatJiraDate(statusChangeTime))
					}
				}
			}
		}

		if (wipTransitionDate.IsZero()) {
			fmt.Printf(TERM_COLOR_RED + "\nNo transition date to WIP found for task %v\n\n" + TERM_COLOR_WHITE, issue.Key)
			continue
		}

		if (resolved) {
			issueTypeMap[issue.Fields.Issuetype.Name]++
		}

		// Task is still in an IDLE column by the end of the selected period
		if isIdle {
			issueDaysInIdle += countWeekDays(idleStart, parameters.EndDate)
		}
		idleDays += issueDaysInIdle

		weekendDays := countWeekendDays(wipTransitionDate, doneTransitionDate)
		issueDaysInWip := round((doneTransitionDate.Sub(wipTransitionDate).Hours() / 24)) - weekendDays

		wipDays += issueDaysInWip

		if parameters.Debug {
			fmt.Printf("\n" + TERM_COLOR_BLUE + "Task: %v - WIP days: %v - Idle days: %v - Start: %v - End: %v", 
				issue.Key, issueDaysInWip, issueDaysInIdle, formatJiraDate(wipTransitionDate), formatJiraDate(doneTransitionDate))

			if resolved {
				fmt.Printf(TERM_COLOR_YELLOW + " (Done)" + TERM_COLOR_WHITE + "\n\n");
			} else {
				fmt.Print(TERM_COLOR_WHITE + "\n\n")
			}
		}
	}

	weekDays := countWeekDays(parameters.StartDate, parameters.EndDate)

	fmt.Printf("Throughput monthly: %v tasks delivered\n", throughtputMonthly)
	fmt.Printf("Throughput weekly: %.2f tasks delivered\n", float64(throughtputMonthly) / float64(4))
	fmt.Printf("Throughput daily: %.2f tasks delivered\n", float64(throughtputMonthly) / float64(weekDays))
	fmt.Printf("Throughput by Issue type:\n")
	for key, value := range issueTypeMap {
		fmt.Printf("- %v: %v\n", key, value)
	}

	fmt.Printf("\nWIP monthly: %v tasks\n", wipMonthly)

	if (wipDays > 0) {
		fmt.Printf("WIP daily: %.2f tasks\n", float64(wipDays) / float64(weekDays))
		if idleDays > 0 { fmt.Printf("Idle days: %v (%v%%)\n", idleDays, ((idleDays * 100) / weekDays)) }
		fmt.Printf("\nLead time: %.2f days\n", float64(wipDays) / float64(throughtputMonthly))
	}
}

func main() {
	var parameters CLParameters = processCommandLineParameters()

	var auth Auth = authenticate(parameters.Login, parameters.JiraUrl)

	boardCfg := loadBoardCfg()

	fmt.Printf("Extracting Kanban metrics from project %v, %v to %v\n\n", 
		boardCfg.Project, formatJiraDate(parameters.StartDate), formatJiraDate(parameters.EndDate))

	extractMetrics(parameters, auth, boardCfg)
}
