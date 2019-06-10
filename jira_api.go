package main

import (
    "bytes"
    "crypto/tls"
    "encoding/json"
    "io/ioutil"
    "net/http"
    "net/url"
    "speakeasy"
    "strconv"
    "strings"
    "time"
)

func authenticate(username string, jiraUrl string) Auth {
    password, err := speakeasy.Ask("Password: ")
    if err != nil {
        panic(err)
    }

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

    if resp.StatusCode != 200 {
        panic("Jira authentication failure")
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

    if insecure {
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
    var searchUrl = jiraUrl + "/rest/api/2/search?jql=" + url.QueryEscape(jql) + "&expand=changelog&maxResults=1000"

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

func countWeekDays(start time.Time, end time.Time) int {
        var weekDays = 0

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
        var weekendDays = 0

        if start.IsZero() {
            return -1
        }

        dateIndex := start
        for dateIndex.Before(end) || dateIndex.Equal(end) {
            if dateIndex.Weekday() == time.Saturday || dateIndex.Weekday() == time.Sunday {
                weekendDays++
            }
            dateIndex = dateIndex.AddDate(0, 0, 1)
        }

        return weekendDays
}

func subtractDatesRemovingWeekends (start time.Time, end time.Time) time.Duration {
    statusChangeDuration := end.Sub(start) 
    weekendDaysBetweenDates := countWeekendDays(start, end)
    if weekendDaysBetweenDates > 0 {
        updatedTotalSeconds := statusChangeDuration.Seconds() - float64(60 * 60 * 24 * weekendDaysBetweenDates)    
        statusChangeDuration = time.Duration(updatedTotalSeconds)*time.Second
    }
    return statusChangeDuration
}

func formatColumns(columns []string) string {
    str := ""

    for index, col := range columns {
        str += "'" + col + "'"
        if index < len(columns) - 1 {
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
