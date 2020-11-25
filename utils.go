package main

import (
	"github.com/andygrunwald/go-jira"
	"io/ioutil"
	"log"
	"math"
	"strings"
	"time"
)

func countWeekDays(start, end time.Time) int {
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

func containsStatus(statusList []string, status string) bool {
	for _, s := range statusList {
		if strings.ToUpper(s) == strings.ToUpper(status) {
			return true
		}
	}
	return false
}

func statusIsNotMapped(status string) bool {
	validStatuses := append(append(append(BoardCfg.OpenStatus, BoardCfg.WipStatus...), BoardCfg.IdleStatus...), BoardCfg.DoneStatus...)
	for _, validStatus := range validStatuses {
		if strings.ToUpper(validStatus) == strings.ToUpper(status) {
			return false
		}
	}
	return true
}

func getIssueTypeByStatus(status string) string {
	if containsStatus(BoardCfg.OpenStatus, status) {
		return "Open"
	} else if containsStatus(BoardCfg.WipStatus, status) {
		return "Wip"
	} else if containsStatus(BoardCfg.IdleStatus, status) {
		return "Idle"
	} else if containsStatus(BoardCfg.DoneStatus, status) {
		return "Done"
	} else {
		return "Not Mapped"
	}
}

func getDays(duration time.Duration) int {
	return int(math.Round(duration.Hours() / 24))
}

func readResponseBody(resp *jira.Response) string {
	if resp != nil {
		body, _ := ioutil.ReadAll(resp.Body)
		return string(body)
	}
	return ""
}

func parseTime(timeStr string) time.Time {
	const layout = "2006-01-02T15:04:05.000-0700"
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		log.Printf("Error parsing date string: %v", err)
	}
	return t
}
