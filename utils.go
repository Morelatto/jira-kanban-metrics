package main

import (
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

func containsStatus(statuses []string, status string) bool {
	for _, s := range statuses {
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

func mergeMaps(map1 map[string][]IssueDetails, map2 map[string][]IssueDetails) map[string][]IssueDetails {
	for issueType, issueDetailsArray := range map2 {
		map1[issueType] = append(map1[issueType], issueDetailsArray...)
	}
	return map1
}

func getDays(duration time.Duration) int {
	return int(math.Round(duration.Hours() / 24))
}
