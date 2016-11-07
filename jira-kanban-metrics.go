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
	"math"
	"bytes"
	"strings"
	"strconv"
	"io/ioutil"
	"net/http"
	"net/url"
	"crypto/tls"
	"encoding/json"
)

// get password lib
import "github.com/bgentry/speakeasy"

func loadBoardCfg() BoardCfg {
	if _, err := os.Stat("jira-board.cfg"); os.IsNotExist(err) {
		panic("jira-board.cfg not found")
	}

	file, _ := os.Open("jira-board.cfg")
	decoder := json.NewDecoder(file)
	boardCfg := BoardCfg{}
	err := decoder.Decode(&boardCfg)
	
	if err != nil {
		fmt.Println("error:", err)
	}

	return boardCfg
}

func authenticate(username string, password string, jiraUrl string) Auth {
	var authUrl = jiraUrl + "/rest/auth/1/session"
	var jsonStr = []byte(`{"username":"` + username + `", "password":"` + password + `"}`)

	req, err := http.NewRequest("POST", authUrl, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var auth Auth
	json.Unmarshal(body, &auth)

	return auth
}

func httpGet(url string, auth Auth, insecure bool) []byte {
	req, err := http.NewRequest("GET", url, nil)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", auth.Session.Name + "=" + auth.Session.Value)

	var client http.Client

	if (insecure) {
		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		client = http.Client{Transport: tr}
	} else {
		client = http.Client{}
	}

	resp, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)	

	if err != nil {
		panic(err)
	}

	return body
}

func searchIssues(jql string, jiraUrl string, auth Auth) SearchResult {
	var searchUrl = jiraUrl + "/rest/api/2/search?jql=" + url.QueryEscape(jql) + "&expand=changelog"

	body := httpGet(searchUrl, auth, true)

	var result SearchResult
	json.Unmarshal(body, &result)

	return result
}

func getIssue(issueId int, jiraUrl string, auth Auth) Issue {
	var issueUrl = jiraUrl + "/rest/api/2/issue/" + strconv.Itoa(issueId) + "?expand=changelog"

	body := httpGet(issueUrl, auth, true)

	var issue Issue
	json.Unmarshal(body, &issue)

	return issue
}

// dateStr: MM/DD/YYYY
func parseDate(dateStr string) time.Time {
	const defaultDateFormat = "01/02/2006"

	parsedDate, err := time.Parse(defaultDateFormat, dateStr)
	if err != nil {
		panic(err)
	}

	return parsedDate
}

func parseJiraTime(timeStr string) time.Time {
	const jiraTimeFormat = "2006-01-02T15:04:05.000-0700"

	parsedTime, err := time.Parse(jiraTimeFormat, timeStr)
	if err != nil {
		panic(err)
	}

	return parsedTime
}

func formatJiraDate(date time.Time) string {
	const jiraDateFormat = "2006/01/02"

	return date.Format(jiraDateFormat)
}

func formatBrDate(date time.Time) string {
	const brDateFormat = "02/01/2006"

	return date.Format(brDateFormat)
}

func stripHours(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func round(number float64) int {
	return int(math.Floor(number + 0.5))
}

func processCommandLineParameters() CLParameters {
	var parameters CLParameters

	if len(os.Args) != 5 {
		fmt.Printf("usage: %v <login> <startDate> <endDate> <jiraUrl>\n", os.Args[0])
		fmt.Printf("example: %v user passwd 01/31/2010 04/31/2010 http://jira.intranet/jira\nfs", os.Args[0])
		os.Exit(0)
	}

	parameters.Login = os.Args[1]
	parameters.StartDate = parseDate(os.Args[2])
	parameters.EndDate = parseDate(os.Args[3])
	parameters.JiraUrl = os.Args[4]

	return parameters
}

func countWeekDays(start time.Time, end time.Time) int {
		var weekDays int = 0

		dateIndex := start
		for dateIndex.Before(end) || dateIndex.Equal(end) { 
			if dateIndex.Weekday() != time.Saturday && dateIndex.Weekday() != time.Sunday {
				weekDays++
			}
			dateIndex = dateIndex.AddDate(0, 0, 1)
		}

		return weekDays
}

func countWeekendDays(start time.Time, end time.Time) int {
		var weekendDays int = 0

		dateIndex := start
		for dateIndex.Before(end) || dateIndex.Equal(end) { 
			if dateIndex.Weekday() == time.Saturday || dateIndex.Weekday() == time.Sunday {
				weekendDays++
			}
			dateIndex = dateIndex.AddDate(0, 0, 1)
		}

		return weekendDays
}

func formatColumns(columns []string) string {
	str := ""

	for index, col := range columns {
		str += "'" + col + "'"
		if (index < len(columns) - 1) {
			str += ","
		}
	}

	return str
}

func containsStatus(statuses []string, status string) bool {
	for _, s := range statuses {
		if strings.ToUpper(s) == strings.ToUpper(status) {
			return true
		}
	}

	return false
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

	troughputSearch := fmt.Sprintf("project = %v AND issuetype != Epic AND status CHANGED TO '%v' DURING('%v', '%v')", 
								   boardCfg.Project, boardCfg.DoneStatus, startDate, endDate)

	result := searchIssues(troughputSearch, parameters.JiraUrl, auth)
	throughtputMonthly := result.Total

	wipSearch := fmt.Sprintf("project = %v AND issuetype != Epic AND (status WAS IN (%v) " + 
							 "DURING('%v', '%v') or status CHANGED TO 'DONE' DURING('2016/10/01', '2016/10/30'))", 
							 boardCfg.Project, formatColumns(boardCfg.WipStatuses), startDate, endDate)

	result = searchIssues(wipSearch, parameters.JiraUrl, auth)
	wipMonthly := result.Total

	var wipDays int = 0

	for _, issue := range result.Issues {
		var start time.Time
		var end time.Time = parameters.EndDate
		var lastDayResolved bool = true
		var resolved bool = false

		for _, history := range issue.Changelog.Histories {
			
			for _, item := range history.Items {

				if item.Field == "status" {
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

					if !start.IsZero() {
						fmt.Printf("%v -> %v (%v) ", item.Fromstring, item.Tostring, formatJiraDate(statusChangeTime))
					}
				}
			}
		}

		if start.IsZero() {
			continue
		}

		weekendDays := countWeekendDays(start, end)
		issueDaysInWip := round((end.Sub(start).Hours() / 24)) - weekendDays

		// if a task Resolved date overlaps the EndDate parameter, it means that the last day should count as a WIP day
		if !lastDayResolved {
			issueDaysInWip++
		}

		wipDays += issueDaysInWip
		fmt.Printf("\n\x1b[94;1mTask: %v - Days on the board: %v - Start: %v - End: %v", issue.Key, issueDaysInWip, formatJiraDate(start), formatJiraDate(end))

		if resolved {
			fmt.Printf(" (Done)\x1b[0m\n\n");
		} else {
			fmt.Print("\x1b[0m\n\n")
		}
	}

	weekDays := countWeekDays(parameters.StartDate, parameters.EndDate)

	fmt.Printf("Throughput monthly: %v tasks delivered\n", throughtputMonthly)
	fmt.Printf("Throughput weekly: %.2f tasks delivered\n", float64(throughtputMonthly) / float64(4))
	fmt.Printf("Throughput daily: %.2f tasks delivered\n", float64(throughtputMonthly) / float64(weekDays))
	fmt.Printf("WIP monthly: %v tasks\n", wipMonthly)
	fmt.Printf("WIP daily: %.2f tasks\n", float64(wipDays) / float64(weekDays))
	fmt.Printf("Lead time: %.2f days\n", float64(wipDays) / float64(throughtputMonthly))
}

type CLParameters struct {
	Login string
	StartDate time.Time
	EndDate time.Time
	JiraUrl string
}

type BoardCfg struct {
	Project string
	WipStatuses []string
	StartStatuses []string
	DoneStatus string
}

type Auth struct {
	Session struct {
		Name string `json:"name"`
		Value string `json:"value"`
	} `json:"session"`
	LoginInfo struct {
		FailedCount int16 `json:"failedCount"`
		LoginCount int16 `json:"loginCount"`
		LastFailedLoginTime string `json:"lastFailedLoginCount"`
		PreviousLoginTime string `json:"previousLoginTime"`
	}	
}

type SearchResult struct {
	Expand string `json:"expand"`
	StartAt int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total int `json:"total"`
	Issues []Issue `json:"issues"`
}

type Issue struct {
	Expand string `json:"expand"`
	ID string `json:"id"`
	Self string `json:"self"`
	Key string `json:"key"`
	Fields struct {
		Resolution interface{} `json:"resolution"`
		Lastviewed string `json:"lastViewed"`
		Aggregatetimeoriginalestimate interface{} `json:"aggregatetimeoriginalestimate"`
		Issuelinks []interface{} `json:"issuelinks"`
		Assignee struct {
			Self string `json:"self"`
			Name string `json:"name"`
			Key string `json:"key"`
			Emailaddress string `json:"emailAddress"`
			Avatarurls struct {
				Four8X48 string `json:"48x48"`
				Two4X24 string `json:"24x24"`
				One6X16 string `json:"16x16"`
				Three2X32 string `json:"32x32"`
			} `json:"avatarUrls"`
			Displayname string `json:"displayName"`
			Active bool `json:"active"`
			Timezone string `json:"timeZone"`
		} `json:"assignee"`
		Subtasks []interface{} `json:"subtasks"`
		Votes struct {
			Self string `json:"self"`
			Votes int `json:"votes"`
			Hasvoted bool `json:"hasVoted"`
		} `json:"votes"`
		Worklog struct {
			Startat int `json:"startAt"`
			Maxresults int `json:"maxResults"`
			Total int `json:"total"`
			Worklogs []interface{} `json:"worklogs"`
		} `json:"worklog"`
		Issuetype struct {
			Self string `json:"self"`
			ID string `json:"id"`
			Description string `json:"description"`
			Iconurl string `json:"iconUrl"`
			Name string `json:"name"`
			Subtask bool `json:"subtask"`
		} `json:"issuetype"`
		Timetracking struct {
		} `json:"timetracking"`
		Environment interface{} `json:"environment"`
		Duedate interface{} `json:"duedate"`
		Timeestimate interface{} `json:"timeestimate"`
		Status struct {
			Self string `json:"self"`
			Description string `json:"description"`
			Iconurl string `json:"iconUrl"`
			Name string `json:"name"`
			ID string `json:"id"`
			Statuscategory struct {
				Self string `json:"self"`
				ID int `json:"id"`
				Key string `json:"key"`
				Colorname string `json:"colorName"`
				Name string `json:"name"`
			} `json:"statusCategory"`
		} `json:"status"`
		Aggregatetimeestimate interface{} `json:"aggregatetimeestimate"`
		Creator struct {
			Self string `json:"self"`
			Name string `json:"name"`
			Key string `json:"key"`
			Emailaddress string `json:"emailAddress"`
			Avatarurls struct {
				Four8X48 string `json:"48x48"`
				Two4X24 string `json:"24x24"`
				One6X16 string `json:"16x16"`
				Three2X32 string `json:"32x32"`
			} `json:"avatarUrls"`
			Displayname string `json:"displayName"`
			Active bool `json:"active"`
			Timezone string `json:"timeZone"`
		} `json:"creator"`
		Timespent interface{} `json:"timespent"`
		Aggregatetimespent interface{} `json:"aggregatetimespent"`
		Workratio int `json:"workratio"`
		Labels []interface{} `json:"labels"`
		Components []interface{} `json:"components"`
		Reporter struct {
			Self string `json:"self"`
			Name string `json:"name"`
			Key string `json:"key"`
			Emailaddress string `json:"emailAddress"`
			Avatarurls struct {
				Four8X48 string `json:"48x48"`
				Two4X24 string `json:"24x24"`
				One6X16 string `json:"16x16"`
				Three2X32 string `json:"32x32"`
			} `json:"avatarUrls"`
			Displayname string `json:"displayName"`
			Active bool `json:"active"`
			Timezone string `json:"timeZone"`
		} `json:"reporter"`
		Progress struct {
			Progress int `json:"progress"`
			Total int `json:"total"`
		} `json:"progress"`
		Project struct {
			Self string `json:"self"`
			ID string `json:"id"`
			Key string `json:"key"`
			Name string `json:"name"`
			Avatarurls struct {
				Four8X48 string `json:"48x48"`
				Two4X24 string `json:"24x24"`
				One6X16 string `json:"16x16"`
				Three2X32 string `json:"32x32"`
			} `json:"avatarUrls"`
		} `json:"project"`
		Resolutiondate interface{} `json:"resolutiondate"`
		Watches struct {
			Self string `json:"self"`
			Watchcount int `json:"watchCount"`
			Iswatching bool `json:"isWatching"`
		} `json:"watches"`
		Updated string `json:"updated"`
		Timeoriginalestimate interface{} `json:"timeoriginalestimate"`
		Description string `json:"description"`
		Summary string `json:"summary"`
		Comment struct {
			Startat int `json:"startAt"`
			Maxresults int `json:"maxResults"`
			Total int `json:"total"`
			Comments []interface{} `json:"comments"`
		} `json:"comment"`
		Fixversions []interface{} `json:"fixVersions"`
		Priority struct {
			Self string `json:"self"`
			Iconurl string `json:"iconUrl"`
			Name string `json:"name"`
			ID string `json:"id"`
		} `json:"priority"`
		Versions []interface{} `json:"versions"`
		Aggregateprogress struct {
			Progress int `json:"progress"`
			Total int `json:"total"`
		} `json:"aggregateprogress"`
		Created string `json:"created"`
		Attachment []interface{} `json:"attachment"`
	} `json:"fields"`
	Changelog struct {
		Startat int `json:"startAt"`
		Maxresults int `json:"maxResults"`
		Total int `json:"total"`
		Histories []struct {
			ID string `json:"id"`
			Author struct {
				Self string `json:"self"`
				Name string `json:"name"`
				Key string `json:"key"`
				Emailaddress string `json:"emailAddress"`
				Avatarurls struct {
					Four8X48 string `json:"48x48"`
					Two4X24 string `json:"24x24"`
					One6X16 string `json:"16x16"`
					Three2X32 string `json:"32x32"`
				} `json:"avatarUrls"`
				Displayname string `json:"displayName"`
				Active bool `json:"active"`
				Timezone string `json:"timeZone"`
			} `json:"author"`
			Created string `json:"created"`
			Items []struct {
				Field string `json:"field"`
				Fieldtype string `json:"fieldtype"`
				From string `json:"from"`
				Fromstring string `json:"fromString"`
				To string `json:"to"`
				Tostring string `json:"toString"`
			} `json:"items"`
		} `json:"histories"`
	} `json:"changelog"`
}
