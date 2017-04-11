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
	"strings"
)

// get password lib
import "github.com/bgentry/speakeasy"

func processCommandLineParameters() CLParameters {
	var parameters CLParameters

	if len(os.Args) < 5 {
		fmt.Printf("usage: %v <login> <startDate> <endDate> <jiraUrl> --debug\n", os.Args[0])
		fmt.Printf("example: %v user passwd 01/31/2010 04/31/2010 http://jira.intranet/jira\nfs", os.Args[0])
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

func main() {
	var parameters CLParameters = processCommandLineParameters()

	boardCfg := loadBoardCfg()

	password, err := speakeasy.Ask("Password: ")
	if err != nil {
		panic(err)
	}

	var auth Auth = authenticate(parameters.Login, password, parameters.JiraUrl)

	startDate := formatJiraDate(parameters.StartDate)
	endDate := formatJiraDate(parameters.EndDate)

	fmt.Printf("Extracting Kanban metrics from project %v, %v to %v\n\n", boardCfg.Project, startDate, endDate)

	troughputSearch := fmt.Sprintf("project = '%v' AND issuetype != Epic AND status CHANGED TO '%v' DURING('%v', '%v')", 
								   boardCfg.Project, boardCfg.DoneStatus, startDate, endDate)

	result := searchIssues(troughputSearch, parameters.JiraUrl, auth)
	throughtputMonthly := result.Total

	wipSearch := fmt.Sprintf("project = '%v' AND issuetype != Epic AND (status WAS IN (%v) " + 
							 "DURING('%v', '%v') or status CHANGED TO '%v' DURING('%v', '%v'))", 
							 boardCfg.Project, formatColumns(boardCfg.WipStatuses), startDate, endDate, boardCfg.DoneStatus, startDate, endDate)

	result = searchIssues(wipSearch, parameters.JiraUrl, auth)
	wipMonthly := result.Total

	var wipDays int = 0
	var idleDays int = 0

	// Transitions on the board: issue -> changelog -> items -> field:status
	for _, issue := range result.Issues {
		
		var start time.Time
		var end time.Time = parameters.EndDate
		
		var idleStart time.Time
		var idleEnd time.Time
		var isIdle bool = false
		var issueDaysInIdle int = 0
		
		var lastDayResolved bool = true
		var resolved bool = false

		for _, history := range issue.Changelog.Histories {
			
			for _, item := range history.Items {

				if item.Field == "status" {
					// Date when the transition happened
					statusChangeTime := stripHours(parseJiraTime(history.Created))

					// FIX: consider only the first change to DEV, a task should not go back on a kanban board
					// the OR operator is to evaluate if a task goes directly from Open to another column different from DEV
					if (containsStatus(boardCfg.StartStatuses, item.Fromstring) || containsStatus(boardCfg.WipStatuses, item.Tostring) && start.IsZero()) {
						start = statusChangeTime
						
						if start.Before(parameters.StartDate) {
							start = parameters.StartDate
						}

					} else if strings.EqualFold(item.Tostring, boardCfg.DoneStatus) {
						end = statusChangeTime
						resolved = true
						
						if end.After(parameters.EndDate) {
							end = parameters.EndDate
							lastDayResolved = false
						}
					}

					// Calculate days on Idle columns
					if containsStatus(boardCfg.IdleStatuses, item.Tostring) {
						fmt.Printf("Tostring = %v\n", item.Tostring)
						idleStart = statusChangeTime
						isIdle = true

					} else if containsStatus(boardCfg.IdleStatuses, item.Fromstring) {
						fmt.Printf("Fromstring = %v\n", item.Fromstring)
						idleEnd = statusChangeTime
						issueDaysInIdle += countWeekDays(idleStart, idleEnd)
						isIdle = false
					}

					if !start.IsZero() && parameters.Debug {
						fmt.Printf("%v -> %v (%v) ", item.Fromstring, item.Tostring, formatJiraDate(statusChangeTime))
					}
				}
			}
		}

		if start.IsZero() {
			continue
		}

		// Task is still in an idle column by the end of the selected period
		if isIdle {
			fmt.Printf("idleStart = %v\n", idleStart)
			issueDaysInIdle += countWeekDays(idleStart, parameters.EndDate)
		}

		idleDays += issueDaysInIdle

		weekendDays := countWeekendDays(start, end)
		issueDaysInWip := round((end.Sub(start).Hours() / 24)) - weekendDays

		// If a task Resolved date overlaps the EndDate parameter, it means that the last day should count as a WIP day
		if !lastDayResolved {
			issueDaysInWip++
		}

		wipDays += issueDaysInWip

		if parameters.Debug {
			fmt.Printf("\n\x1b[94;1mTask: %v - Days on the board: %v - Idle days: %v - Start: %v - End: %v", 
				issue.Key, issueDaysInWip, issueDaysInIdle, formatJiraDate(start), formatJiraDate(end))
			
			if resolved {
				fmt.Printf(" (Done)\x1b[0m\n\n");
			} else {
				fmt.Print("\x1b[0m\n\n")
			}
		}
	}

	weekDays := countWeekDays(parameters.StartDate, parameters.EndDate)

	fmt.Printf("Throughput monthly: %v tasks delivered\n", throughtputMonthly)
	fmt.Printf("Throughput weekly: %.2f tasks delivered\n", float64(throughtputMonthly) / float64(4))
	fmt.Printf("Throughput daily: %.2f tasks delivered\n", float64(throughtputMonthly) / float64(weekDays))
	fmt.Printf("WIP monthly: %v tasks\n", wipMonthly)
	fmt.Printf("WIP daily: %.2f tasks\n", float64(wipDays) / float64(weekDays))
	if idleDays > 0 { fmt.Printf("Idle days: %v (%v%%)\n", idleDays, ((idleDays * 100) / wipDays)) }
	fmt.Printf("Lead time: %.2f days\n", float64(wipDays) / float64(throughtputMonthly))
}