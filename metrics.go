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
			toPrint := color.RedString(issueDetails.Name) + separator
			toPrint += color.WhiteString(issueDetails.Summary) + separator
			toPrint += color.YellowString("Created: %s", formatBrDate(issueDetails.CreatedDate))

			if issueDetails.ToWipDate != issueDetails.CreatedDate {
				toPrint += separator
				toPrint += color.YellowString("To WIP: %s", formatBrDate(issueDetails.ToWipDate))
			}

			if issueDetails.Resolved {
				toPrint += separator
				toPrint += color.YellowString("Resolved: %s", formatBrDate(issueDetails.ResolvedDate))
			}

			if issueDetails.WIP.Hours() > 1 {
				toPrint += separator

				var wipDays int
				if issueDetails.WIP.Hours() < 24 {
					wipDays = 1
				} else {
					wipDays = int(math.Round(issueDetails.WIP.Hours() / 24))
				}
				toPrint += color.WhiteString("WIP: %d", wipDays)
			}

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
			if len(issueDetails.CustomFields) > 0 {
				toPrint += separator
				toPrint += color.CyanString("Custom Fields: %v", strings.Join(issueDetails.CustomFields, ", "))
			}
			if issueDetails.LastStatus != "" {
				toPrint += color.YellowString(" (%s)", issueDetails.LastStatus)
			}
			toPrint += "\n"
			_, _ = fmt.Fprintf(color.Output, toPrint)
		}
	}
}

func printAverageByStatus(issueDetailsMapByType map[string][]IssueDetails) {
	totalDurationByStatusMap := make(map[string]time.Duration)
	var totalDuration time.Duration
	for _, issueDetailsArray := range issueDetailsMapByType {
		for _, issueDetails := range issueDetailsArray {
			for status, duration := range issueDetails.DurationByStatus {
				totalDurationByStatusMap[status] += duration
				totalDuration += duration
			}
		}
	}
	title("\n> Average by Status\n")
	for status, duration := range totalDurationByStatusMap {
		statusPercent := float64(duration*100) / float64(totalDuration)
		fmt.Printf("%v = %.2f%%", status, statusPercent)
		warn(" [%s]\n", durafmt.Parse(duration))
	}
}

func printAverageByStatusType(issueDetailsMapByType map[string][]IssueDetails) {
	var totalDuration time.Duration
	totalDurationByStatusTypeMap := make(map[string]time.Duration)
	for _, issueDetailsArray := range issueDetailsMapByType {
		for _, issueDetails := range issueDetailsArray {
			for status, duration := range issueDetails.DurationByStatus {
				totalDurationByStatusTypeMap[getIssueTypeByStatus(status)] += duration
				totalDuration += duration
			}
		}
	}
	title("\n> Average by Status Type\n")
	for statusType, duration := range totalDurationByStatusTypeMap {
		statusPercent := float64(duration*100) / float64(totalDuration)
		fmt.Printf("%v = %.2f%%", statusType, statusPercent)
		warn(" [%s]\n", durafmt.Parse(duration))
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
	fmt.Printf("Monthly: ")
	warn("%d tasks\n", wipMonthly)
	totalWipDays := totalWipDuration.Hours() / 24
	fmt.Printf("Average: ")
	warn("%.2f tasks\n", totalWipDays/float64(weekDays))
}

func printThroughput(issueDetailsMapByType map[string][]IssueDetails) {
	var totalThroughput int
	throughputMap := make(map[string]int)
	for issueType, issueDetailsArray := range issueDetailsMapByType {
		for _, issueDetails := range issueDetailsArray {
			if issueDetails.Resolved {
				throughputMap[issueType]++
				totalThroughput++
			}
		}
	}
	title("\n> Throughput\n")
	fmt.Printf("Total: ")
	warn("%d tasks delivered\n", totalThroughput)
	fmt.Printf("By issue type:\n")
	for issueType, throughput := range throughputMap {
		fmt.Printf("- %v: %v tasks", issueType, throughput)
		warn(" (%d%%)\n", (throughput*100)/totalThroughput)
	}
}

func printLeadTime(issueDetailsMapByType map[string][]IssueDetails) {
	var throughputMonthly int
	var totalWipDuration time.Duration
	var leadTimeByTypeMap = make(map[string]float64)

	for issueType, issueDetailsArray := range issueDetailsMapByType {
		var wipByType time.Duration
		for _, issueDetails := range issueDetailsArray {
			wipByType += issueDetails.WIP
		}
		typeThroughput := len(issueDetailsArray)
		throughputMonthly += typeThroughput
		totalWipDuration += wipByType
		leadTimeByTypeMap[issueType] = float64(wipByType) / float64(typeThroughput)
	}

	title("\n> Lead time\n")
	fmt.Printf("Average: ")
	wipDays := int(math.Round(totalWipDuration.Hours() / 24))
	warn("%d days\n", wipDays/throughputMonthly)
	fmt.Printf("By issue type:\n")
	for issueType, leadTime := range leadTimeByTypeMap {
		fmt.Printf("- %v: ", issueType)
		warn("%v days\n", math.Round(time.Duration(leadTime).Hours()/24))
	}
}
