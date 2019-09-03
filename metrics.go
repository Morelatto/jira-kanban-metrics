package main

import (
	"fmt"
	"github.com/hako/durafmt"
	"github.com/zchee/color" // TODO test colors on windows and terminator
	"math"
	"strings"
	"time"
)

func printIssueDetailsByType(issueDetailsMapByType map[string][]IssueDetails) {
	const separator = " | "
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		title("\n>> %s\n", issueType)
		for _, issueDetails := range issueDetailsArray {
			startDate, endDate := formatBrDate(issueDetails.StartDate), formatBrDate(issueDetails.EndDate)
			toPrint := color.RedString(issueDetails.Name) + separator
			toPrint += color.WhiteString(issueDetails.Summary) + separator
			toPrint += color.YellowString("Start: %s", startDate) + separator
			toPrint += color.YellowString("End: %s", endDate) + separator
			toPrint += color.WhiteString("WIP: %s", durafmt.Parse(getTotalWip(issueDetails.DurationByStatus)))
			if issueDetails.EpicLink != "" {
				toPrint += separator
				toPrint += color.GreenString("Epic link: %v", issueDetails.EpicLink)
			}
			if len(issueDetails.Labels) > 0 {
				toPrint += separator
				toPrint += color.BlueString("Labels: %v", strings.Join(issueDetails.Labels, ", "))
			}
			if issueDetails.Sprint != "" {
				toPrint += separator
				toPrint += color.GreenString("Sprint: %v", issueDetails.Sprint)
			}
			if issueDetails.Resolved {
				toPrint += color.YellowString(" (Done)")
			}
			toPrint += "\n"
			_, _ = fmt.Fprintf(color.Output, toPrint)
		}
	}
}

func getTotalWip(durationByStatus map[string]time.Duration) time.Duration {
	var totalWip time.Duration
	for status, duration := range durationByStatus {
		if getIssueTypeByStatus(status) == "Wip" {
			totalWip += duration
		}
	}
	return totalWip
}

func printAverageByStatus(issueDetailsMapByType map[string][]IssueDetails, totalWipDuration time.Duration) {
	totalDurationByStatusMap := make(map[string]time.Duration)
	for _, issueDetailsArray := range issueDetailsMapByType {
		for _, issueDetails := range issueDetailsArray {
			for status, duration := range issueDetails.DurationByStatus {
				totalDurationByStatusMap[status] += duration
			}
		}
	}
	title("\n> Average by Status\n")
	for status, totalDuration := range totalDurationByStatusMap {
		statusPercent := float64(totalDuration*100) / float64(totalWipDuration)
		fmt.Printf("%v = %.2f%%\n", status, statusPercent)
	}
}

func printAverageByStatusType(issueDetailsMapByType map[string][]IssueDetails, totalWipDuration time.Duration) {
	totalDurationByStatusTypeMap := make(map[string]time.Duration)
	for _, issueDetailsArray := range issueDetailsMapByType {
		for _, issueDetails := range issueDetailsArray {
			for status, duration := range issueDetails.DurationByStatus {
				totalDurationByStatusTypeMap[getIssueTypeByStatus(status)] += duration
			}
		}
	}
	title("\n> Average by Status Type\n")
	for statusType, totalDuration := range totalDurationByStatusTypeMap {
		statusPercent := float64(totalDuration*100) / float64(totalWipDuration)
		fmt.Printf("%v = %.2f%% [%v] \n", statusType, statusPercent, totalDuration)
	}
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

func printWIP(totalWipDuration time.Duration, wipMonthly, weekDays int) {
	title("\n> WIP\n")
	fmt.Printf("Monthly: %v tasks\n", wipMonthly)
	totalWipDays := totalWipDuration.Hours() / 24
	fmt.Printf("Average: %.2f tasks\n", totalWipDays/float64(weekDays))
}

func printThroughput(throughputMonthly int, issueDetailsMapByType map[string][]IssueDetails) {
	title("\n> Throughput\n")
	fmt.Printf("Total: %v tasks delivered\n", throughputMonthly)
	fmt.Printf("By issue type:\n")
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		issueCount := len(issueDetailsArray)
		fmt.Printf("- %v: %v tasks (%v%%)\n", issueType, issueCount, (issueCount*100)/throughputMonthly)
	}
}

func printLeadTime(totalWipDuration time.Duration, throughputMonthly int, issueDetailsMapByType map[string][]IssueDetails) {
	var issueTypeLeadTimeMap = make(map[string]float64)
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		var wipDays []float64
		var totalWipByType time.Duration
		for _, issueDetails := range issueDetailsArray {
			issueWip := getTotalWip(issueDetails.DurationByStatus)
			totalWipByType += issueWip
			wipDays = append(wipDays, float64(issueWip))
		}
		totalWipAverageByIssueType := float64(totalWipByType) / float64(len(issueDetailsArray))
		issueTypeLeadTimeMap[issueType] = totalWipAverageByIssueType
	}

	title("\n> Lead time\n")
	totalWipDays := totalWipDuration.Hours() / 24
	fmt.Printf("Average: %v days\n", math.Round(totalWipDays/float64(throughputMonthly)))
	fmt.Printf("By issue type:\n")
	for issueType, leadTime := range issueTypeLeadTimeMap {
		fmt.Printf("- %v: %v days\n", issueType, math.Round(time.Duration(leadTime).Hours()/24))
	}
}
