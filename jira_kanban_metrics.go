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

    // Add one day to end date limit to include it in time comparisons
    parameters.EndDate = parameters.EndDate.Add(time.Hour * 24)

    var wipDays int = 0 // Absolute number of WIP days of all issues during the specified period
    var directResolvedIssues int = 0 // Absolute number of direct resolved issues (from OPEN to DONE)
    var issueTypeMap map[string]int = make(map[string]int) // Number of issues by type [key]
    var totalDurationMap map[string]float64 = make(map[string]float64) // Total duration [value] by status [key] of all issues

    // Transitions on the board: Issue -> Changelog -> Histories -> Items -> Field:Status
    for _, issue := range result.Issues {

        var wipTransitionDate time.Time
        var doneTransitionDate time.Time = parameters.EndDate

        var resolved bool = false
        var ignoreIssue bool = false

        var durationMap map[string]int64 = make(map[string]int64)  // Total duration [value] by status [key]
        var statusChangeMap map[string]time.Time = make(map[string]time.Time) // Maps when a transition to status [key] happened

        for _, history := range issue.Changelog.Histories {
            for _, item := range history.Items {
                if item.Field == "status" {

                    // Timestamp when the transition happened
                    statusChangeTime := parseJiraTime(history.Created)

                    if (containsStatus(boardCfg.WipStatus, item.Tostring)) {
                        _, ok := statusChangeMap[item.Tostring]
                        if ok {
                            fmt.Printf(TERM_COLOR_RED + "Issue %v - Transition TO issue %v happened twice before a corresponding FROM transition was found, ignoring transition\n" + TERM_COLOR_WHITE, 
                                issue.Key, item.Tostring)
                            continue
                        }
                        if statusChangeTime.Before(parameters.StartDate) {
                            statusChangeMap[item.Tostring] = parameters.StartDate
                        } else if statusChangeTime.After(parameters.EndDate) {
                            statusChangeMap[item.Tostring] = parameters.EndDate
                        } else {
                            statusChangeMap[item.Tostring] = statusChangeTime
                        }
                    }

                    if (containsStatus(boardCfg.WipStatus, item.Fromstring)) {
                        fromDate := parameters.EndDate
                        toDate, ok := statusChangeMap[item.Fromstring]
                        if !ok {
                            fmt.Printf(TERM_COLOR_RED + "Issue %v - Transition FROM issue %v doesn't have a corresponding TO transition\n" + TERM_COLOR_WHITE,
                                issue.Key, item.Fromstring)
                            continue
                        }
                        delete(statusChangeMap, item.Fromstring)
                        if !statusChangeTime.Before(parameters.StartDate) {
                            if !statusChangeTime.After(parameters.EndDate) {
                                fromDate = statusChangeTime
                            }
                            duration := int64(fromDate.Sub(toDate))
                            durationMap[item.Fromstring] += duration
                        }
                    }

                    // Transition from OPEN to WIP
                    if (containsStatus(boardCfg.OpenStatus, item.Fromstring) && containsStatus(boardCfg.WipStatus, item.Tostring)) {
                        if (statusChangeTime.After(parameters.EndDate)) {
                            fmt.Printf(TERM_COLOR_RED + "Issue %v - Transition to WIP happened after end date: %v, ignoring issue\n" + TERM_COLOR_WHITE, issue.Key, statusChangeTime)
                            ignoreIssue = true
                        }

                        wipTransitionDate = statusChangeTime
                        doneTransitionDate = parameters.EndDate
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

                    // Transition from WIP to OPEN
                    if (containsStatus(boardCfg.WipStatus, item.Fromstring) && containsStatus(boardCfg.OpenStatus, item.Tostring)) {
                        doneTransitionDate = parameters.EndDate

                        if statusChangeTime.Before(parameters.EndDate) || statusChangeTime.Equal(parameters.EndDate) {
                            doneTransitionDate = statusChangeTime
                        }
                    }

                    // Transition from OPEN to DONE
                    if (containsStatus(boardCfg.OpenStatus, item.Fromstring) && containsStatus(boardCfg.DoneStatus, item.Tostring)) {
                        wipTransitionDate = statusChangeTime
                        doneTransitionDate = statusChangeTime
                        directResolvedIssues++
                        resolved = true

                        if wipTransitionDate.Before(parameters.StartDate) {
                            wipTransitionDate = parameters.StartDate
                        }
                        if doneTransitionDate.After(parameters.EndDate) {
                            doneTransitionDate = parameters.EndDate
                        }
                    }

                    // Log debug the transition
                    if parameters.Debug {
                        fmt.Printf("%v -> %v (%v)\n", item.Fromstring, item.Tostring, formatJiraDate(statusChangeTime))
                    }
                }
            }
        }

        // Count duration of last transition until the end of the specified period
        if len(statusChangeMap) == 1 {
            for status, toDate := range statusChangeMap {
                fromDate := parameters.EndDate
                duration := int64(fromDate.Sub(toDate))
                durationMap[status] += duration
            }

        } else if len(statusChangeMap) > 1 {
            fmt.Printf(TERM_COLOR_RED + "Issue %v - Status change map state is inconsistent: %v\n" + TERM_COLOR_WHITE, issue.Key, statusChangeMap)
        }

        if ignoreIssue {
            wipMonthly--
            continue
        }


        if (wipTransitionDate.IsZero()) {
            fmt.Printf(TERM_COLOR_RED + "Issue %v - No transition date to WIP found\n" + TERM_COLOR_WHITE, issue.Key)
            continue
        }

        if (resolved) {
            issueTypeMap[issue.Fields.Issuetype.Name]++
        }

        weekendDays := countWeekendDays(wipTransitionDate, doneTransitionDate)
        issueDaysInWip := round(doneTransitionDate.Sub(wipTransitionDate).Hours() / 24) - weekendDays

        wipDays += issueDaysInWip

        periodDuration := int64(doneTransitionDate.Sub(wipTransitionDate))
        if periodDuration > 0 {
            var total float64 = 0

            for k, v := range durationMap {
                periodPercent := float64(v * 100) / float64(periodDuration)
                if periodPercent >= 0.01 {
                    if parameters.Debug {
                        fmt.Printf("%v = %.2f%%\n", k, periodPercent)
                    }
                    totalDurationMap[k] += periodPercent
                    total += periodPercent
                }
            }

            if total < 99.9 {
                fmt.Printf(TERM_COLOR_RED + "Issue %v - Average by status %.2f%% total is less than 100%%\n" + TERM_COLOR_WHITE, issue.Key, total)
            }
        }

        fmt.Printf(TERM_COLOR_BLUE + "Issue: %v - %v - WIP days: %v - Start: %v - End: %v", 
            issue.Key, issue.Fields.Summary, issueDaysInWip, formatJiraDate(wipTransitionDate), formatJiraDate(doneTransitionDate))

        if resolved {
            fmt.Printf(TERM_COLOR_YELLOW + " (Done)" + TERM_COLOR_WHITE + "\n\n")
        } else {
            fmt.Print(TERM_COLOR_WHITE + "\n\n")
        }
    }

    weekDays := countWeekDays(parameters.StartDate, parameters.EndDate)

    if wipDays > 0 {
        var totalIdle float64 = 0
        fmt.Printf("> Average by Status\n")

        for k, v := range totalDurationMap {
            percent := v / float64(wipMonthly - directResolvedIssues)
            fmt.Printf("- %v: %.2f%%", k, percent)
            if containsStatus(boardCfg.IdleStatus, k) {
                totalIdle += percent
                fmt.Printf(" (Idle)\n")
            } else {
                fmt.Printf("\n")
            }
        }

        if totalIdle > 0 {
            fmt.Printf("- Idle Total: %.2f%%\n", totalIdle)
        }
    }

    fmt.Printf("\n> Throughput\n")
    fmt.Printf("Monthly: %v tasks delivered\n", throughtputMonthly)
    fmt.Printf("Weekly: %.2f tasks\n", float64(throughtputMonthly) / float64(4))
    fmt.Printf("Daily: %.2f tasks\n", float64(throughtputMonthly) / float64(weekDays))
    fmt.Printf("By issue type:\n")
    for key, value := range issueTypeMap {
        fmt.Printf("- %v: %v tasks (%v%%)\n", key, value, ((value * 100) / throughtputMonthly))
    }

    fmt.Printf("\n> WIP\n")
    fmt.Printf("Monthly: %v tasks\n", wipMonthly)
    if wipDays > 0 {
        fmt.Printf("Average: %.2f tasks\n", float64(wipDays) / float64(weekDays))
    }

    fmt.Printf("\n> Lead time: %.2f days\n", float64(wipDays) / float64(throughtputMonthly))
}

func main() {
    var parameters CLParameters = processCommandLineParameters()

    var auth Auth = authenticate(parameters.Login, parameters.JiraUrl)

    boardCfg := loadBoardCfg()

    fmt.Printf("Extracting Kanban metrics from project %v, %v to %v\n\n", 
        boardCfg.Project, formatJiraDate(parameters.StartDate), formatJiraDate(parameters.EndDate))

    extractMetrics(parameters, auth, boardCfg)
}
