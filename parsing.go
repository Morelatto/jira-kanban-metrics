package main

import (
	"time"
)

// dateStr: DD/MM/YYYY
func parseDate(dateStr string) time.Time {
	const defaultDateFormat = "02/01/2006"

	parsedDate, err := time.Parse(defaultDateFormat, dateStr)
	if err != nil {
		return time.Time{}
	}

	return parsedDate
}

func formatJiraDate(date time.Time) string {
	const jiraDateFormat = "2006/01/02"
	return date.Format(jiraDateFormat)
}

func formatBrDate(date time.Time) string {
	const brDateFormat = "02/01/2006"
	return date.Format(brDateFormat)
}

func formatBrDateWithTime(date time.Time) string {
	const brDateFormat = "02/01/2006 15:04:05"
	return date.Format(brDateFormat)
}
