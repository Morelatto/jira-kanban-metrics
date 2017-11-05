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
    "strconv"
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

    wipSearch := fmt.Sprintf("project = '%v' AND  issuetype != Epic " + 
                             // "AND issue = MLG-312 " + 
                             // "AND issue = MLG-353 " + 
                             "AND issue = MLG-335    " + 
                             // "AND (status WAS IN (%v) DURING('%v', '%v') " + 
                             "AND (status CHANGED TO %v DURING('%v', '%v'))",                              
                             boardCfg.Project, 
                             // formatColumns(boardCfg.WipStatus), startDate, endDate, 
                             formatColumns(boardCfg.DoneStatus), startDate, endDate)

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

        // var wipTransitionDate time.Time
        // var doneTransitionDate time.Time = parameters.EndDate

        var resolved bool = false
        // var ignoreIssue bool = false
        var hasAlreadyFoundLastStatusChange bool = false


        // var durationMap map[string]int64 = make(map[string]int64)  // Total duration [value] by status [key]
        // var statusChangeMap map[string]time.Time = make(map[string]time.Time) // Maps when a transition to status [key] happened



        var durationByStatusMap map[string]int64 = make(map[string]int64)  // Total duration [value] by status [key]
        var durationByStatusTypeMap map[string]int64 = make(map[string]int64)  // Total duration [value] by status [key]
        var issueDurationByStatysMap map[string]time.Duration = make(map[string]time.Duration)  // Total duration [value] by status [key]
        var totalMinutesByIssue int64

        var epicLink string

        var lastFromStatus string
        var lastToStatus string
        var lastFromStatusCreationDate time.Time

        var transitionToWipDate time.Time

        for _, history := range issue.Changelog.Histories {

            for _, item := range history.Items {

                if item.Field == "status" {

                    // ignoreIssue if has already found the last one inside the period
                    if (hasAlreadyFoundLastStatusChange) {
                        continue
                    }

                    // Timestamp when the transition happened
                    statusChangeTime := parseJiraTime(history.Created)
                    if (statusChangeTime.Before(parameters.StartDate)) {
                        if parameters.Debug {                
                            fmt.Printf(TERM_COLOR_RED + "Status changed [%v] before initial period [%v], considering stardDate \n" + TERM_COLOR_WHITE, formatBrDateWithTime(statusChangeTime), formatBrDateWithTime(parameters.StartDate))
                        }
                        statusChangeTime = parameters.StartDate
                    }

                    if (statusChangeTime.After(parameters.EndDate)) {
                        if parameters.Debug {
                            fmt.Printf(TERM_COLOR_RED + "Status changed [%v] after end period [%v], considering endDate \n" + TERM_COLOR_WHITE, formatBrDateWithTime(statusChangeTime), formatBrDateWithTime(parameters.StartDate))
                        }
                        statusChangeTime = parameters.EndDate
                        hasAlreadyFoundLastStatusChange = true
                    }

                    if (statusChangeTime.Before(parameters.StartDate) || statusChangeTime.After(parameters.EndDate)) {
                        //update vars for next interation
                        lastFromStatus = item.Fromstring
                        lastToStatus = item.Tostring
                        lastFromStatusCreationDate = statusChangeTime
                        continue
                    }

                    // Get the first status date, and continue to the next status transition
                    if (lastFromStatus == "") {
                        lastFromStatus = item.Fromstring                        
                        lastFromStatusCreationDate = statusChangeTime
                        if parameters.Debug {
                            fmt.Printf(TERM_COLOR_WHITE + "First Status [%v] Created in [%v] \n\n", lastFromStatus, lastFromStatusCreationDate)
                        }
                        continue
                    }

                    // Mapping var to calculate total WIP of the issue
                    if (transitionToWipDate.IsZero() && (
                        containsStatus(boardCfg.WipStatus, item.Tostring) || containsStatus(boardCfg.WipStatus, item.Tostring) ||
                        containsStatus(boardCfg.WipStatus, item.Fromstring) || containsStatus(boardCfg.WipStatus, item.Fromstring))) {
                        transitionToWipDate = statusChangeTime;
                        fmt.Printf(TERM_COLOR_RED + "TransitionToWip happened in [%v] \n" + TERM_COLOR_WHITE, formatBrDateWithTime(statusChangeTime))
                    }

                    // Calculating status transition duration
                    statusChangeDuration := statusChangeTime.Sub(lastFromStatusCreationDate) 
                    weekendDaysBetweenDates := countWeekendDays(lastFromStatusCreationDate, statusChangeTime)
                    if (weekendDaysBetweenDates > 0) {
                        updatedTotalSeconds := statusChangeDuration.Seconds() - float64(60 * 60 * 24 * weekendDaysBetweenDates)    
                        statusChangeDuration = time.Duration(updatedTotalSeconds)*time.Second
                        if parameters.Debug {
                            fmt.Printf(TERM_COLOR_RED + "Removing weekend days [%v] from Status [%v] \n" + TERM_COLOR_WHITE, weekendDaysBetweenDates, item.Fromstring)
                        }
                    }

                    if parameters.Debug {
                        printDebugIssueTransition (parameters.Debug, statusChangeTime, lastFromStatusCreationDate, statusChangeDuration, item.Fromstring, item.Tostring) 
                    }
                    
                    // increment total minutes of this status transition
                    totalMinutesByIssue += int64(statusChangeDuration.Minutes())

                    // Group total minutes by status, considering this status transition
                    durationByStatusMap[item.Fromstring] = durationByStatusMap[item.Fromstring] + int64(statusChangeDuration.Minutes())
                    issueDurationByStatysMap[item.Fromstring] = issueDurationByStatysMap[item.Fromstring] + statusChangeDuration

                    //update vars for next interation
                    lastFromStatus = item.Fromstring
                    lastToStatus = item.Tostring
                    lastFromStatusCreationDate = statusChangeTime

                } else if item.Field == "Epic Link" {
                    epicLink = item.Tostring
                }
            }
        }

        // Calculate the duration of the last transition, if it's not done
        if (lastFromStatusCreationDate.Before(parameters.EndDate) && !containsStatus(boardCfg.DoneStatus, lastToStatus)) {
            statusChangeDuration := parameters.EndDate.Sub(lastFromStatusCreationDate)

           // increment total minutes of this status transition
            totalMinutesByIssue += int64(statusChangeDuration.Minutes())

            // Group total minutes by status, considering this status transition          
            durationByStatusMap[lastToStatus] = durationByStatusMap[lastToStatus] + int64(statusChangeDuration.Minutes())
            issueDurationByStatysMap[lastToStatus] = issueDurationByStatysMap[lastToStatus] + statusChangeDuration
            
            // print debug
            if parameters.Debug {                
                fmt.Printf(TERM_COLOR_RED + "Status current in development, considering endDate [%v] \n" + TERM_COLOR_WHITE, formatBrDateWithTime(parameters.EndDate))
            }   
            printDebugIssueTransition (parameters.Debug, parameters.EndDate, lastFromStatusCreationDate, statusChangeDuration, lastToStatus, "None") 
        }

        // Verify if the last transition is to a resolved status
        if (containsStatus(boardCfg.DoneStatus, lastToStatus)) {
            resolved = true
            issueTotalWip := subDatesRemovingWeekends(true, transitionToWipDate, lastFromStatusCreationDate) 
            fmt.Printf(TERM_COLOR_RED + "Issue total wip [%v] \n" + TERM_COLOR_WHITE, issueTotalWip)                        
        }

        fmt.Printf("\n")

        var totalDuration time.Duration
        for k, v := range issueDurationByStatysMap {  
            totalDuration = totalDuration + v 
            if parameters.Debug {
                statusPercent := float64(v * 100) / float64(totalMinutesByIssue)
                fmt.Printf("%v = %.2f%% [%v] \n", k, statusPercent, v)
            }
        }

        fmt.Printf("TOTAL DURATION: [%v]\n", totalDuration)

        fmt.Printf("\n")

        // grouping by status type configured in board.cfg
        var statusType string
        for k, v := range durationByStatusMap {        
            if (containsStatus(boardCfg.OpenStatus, k)) {
                statusType = "Open";
            } else if (containsStatus(boardCfg.WipStatus, k)) {
                statusType = "Wip";
            } else if (containsStatus(boardCfg.IdleStatus, k)) {
                statusType = "Idle";
            } else if (containsStatus(boardCfg.DoneStatus, k)) {
                statusType = "Done";
            } else {
                fmt.Printf("%v = not mapped in board.cfg, please update it.\n", k)
                continue
            }

            durationByStatusTypeMap[statusType] = durationByStatusTypeMap[statusType] + v

            if parameters.Debug {
                statusPercent := float64(v * 100) / float64(totalMinutesByIssue) 
                fmt.Printf("%v = %.2f%% [%v minutes] \n", k, statusPercent, v)
            }
        }

        fmt.Printf("\n")

        // calculating percentage by status type configured in board.cfg
        for k, v := range durationByStatusTypeMap {
            statusPercent := float64(v * 100) / float64(totalMinutesByIssue) 
            fmt.Printf("%v = %.2f%% [%v minutes] \n", k, statusPercent, v)
        }


        if (resolved) {
            issueTypeMap[issue.Fields.Issuetype.Name]++
        }

        fmt.Printf(TERM_COLOR_BLUE + "Issue Jira: %v | %v | WIP days: %v | ", 
            issue.Key, issue.Fields.Summary, durationByStatusTypeMap["Wip"]/(24*60))

        if epicLink != "" {
            fmt.Printf(" Epic link: %v |", epicLink)
        }

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

func printDebugIssueTransition (isDebug bool, statusChangeTime time.Time, statusChangeTimeStart time.Time, statusChangeDuration time.Duration, statusFrom string, statusTo string) {

    if isDebug {
    
        // Calculating days, hours and minutes of this status transition
        statusChangeDurationDays := int(statusChangeDuration.Hours())/int(24)
        statusChangeDurationHours := int(statusChangeDuration.Hours() - float64(statusChangeDurationDays*int(24)))
        statusChangeDurationMinutes := int(statusChangeDuration.Minutes()- float64((statusChangeDurationDays*24*60)+(statusChangeDurationHours*int(60))))

        // printing this data
        // fmt.Printf("%v -> %v (%v)\n", statusFrom, statusTo, formatJiraDate(statusChangeTime))
        fmt.Printf("%v -> %v (%v)\n", statusFrom, statusTo, formatBrDateWithTime(statusChangeTime))
        fmt.Printf("Status [%v] Time in Status [%vd %vh %vm] \n", statusFrom, statusChangeDurationDays, statusChangeDurationHours, statusChangeDurationMinutes)
        fmt.Printf("Debug [%v] - [%v] = [%v] \n\n", formatBrDateWithTime(statusChangeTime), formatBrDateWithTime(statusChangeTimeStart), statusChangeDuration)
    }
}

func printDurationInDays (isDebug bool, statusChangeDuration time.Duration) string {

    if isDebug {
    
        // Calculating days, hours and minutes of this status transition
        statusChangeDurationDays := int(statusChangeDuration.Hours())/int(24)
        // statusChangeDurationHours := int(statusChangeDuration.Hours() - float64(statusChangeDurationDays*int(24)))
        // statusChangeDurationMinutes := int(statusChangeDuration.Minutes()- float64((statusChangeDurationDays*24*60)+(statusChangeDurationHours*int(60))))
        returnString := "" + strconv.Itoa(statusChangeDurationDays) + "d "
         // + statusChangeDurationHours+ "h" + statusChangeDurationMinutes + "m"
        return returnString 
    }

    return ""
}


func subDatesRemovingWeekends (isDebug bool, start time.Time, end time.Time) time.Duration {
    statusChangeDuration := end.Sub(start) 
    weekendDaysBetweenDates := countWeekendDays(start, end)
    if (weekendDaysBetweenDates > 0) {
        updatedTotalSeconds := statusChangeDuration.Seconds() - float64(60 * 60 * 24 * weekendDaysBetweenDates)    
        statusChangeDuration = time.Duration(updatedTotalSeconds)*time.Second
        if isDebug {
            fmt.Printf(TERM_COLOR_RED + "Removing weekend days [%v] \n" + TERM_COLOR_WHITE, weekendDaysBetweenDates)
        }
    }
    return statusChangeDuration
}


