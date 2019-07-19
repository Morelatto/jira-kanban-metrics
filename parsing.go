package main

import (
	"time"
)

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
	return parsedTime.UTC()
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
	const brDateFormat = "02/01/2006 15:04"

	return date.Format(brDateFormat)
}
