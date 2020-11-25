package main

import (
	"fmt"
	"github.com/hako/durafmt"
	"github.com/zchee/color"
	"strings"
	"time"
)

func printIssueDetailsByType(issueDetailsMapByType map[string][]IssueDetails) {
	const separator = " | "
	for issueType, issueDetails := range issueDetailsMapByType {
		title("\n>> %s\n", issueType)
		for _, issueDetails := range issueDetails {
			toPrint := color.RedString(issueDetails.Key) + separator
			toPrint += color.WhiteString(issueDetails.Title) + separator
			toPrint += color.YellowString("Created: %s", formatBrDate(issueDetails.CreatedDate))

			if !issueDetails.WipDate.IsZero() {
				toPrint += separator
				toPrint += color.YellowString("To WIP: %s", formatBrDate(issueDetails.WipDate))
			}

			if !issueDetails.ResolvedDate.IsZero() {
				toPrint += separator
				toPrint += color.YellowString("Resolved: %s", formatBrDate(issueDetails.ResolvedDate))
			}

			if wipIdle := issueDetails.GetWipAndIdleTotalDuration(); wipIdle > 1 {
				toPrint += separator

				var days int
				if wipIdle.Hours() < 24 {
					days = 1
				} else {
					days = getDays(wipIdle)
				}
				toPrint += color.WhiteString("WIP/Idle: %d", days)
			}

			if wip := issueDetails.GetWipTotalDuration(); wip > 1 {
				toPrint += separator

				var days int
				if wip.Hours() < 24 {
					days = 1
				} else {
					days = getDays(wip)
				}
				toPrint += color.WhiteString("WIP: %d", days)
			}

			if len(issueDetails.FlagDetails) != 0 {
				totalFlagDays := 0
				for _, flag := range issueDetails.FlagDetails {
					if flagDuration := flag.GetFlagDuration(); flagDuration.Hours() >= 4 {
						flagDays := getDays(flagDuration)
						if flagDays < 1 {
							flagDays = 1
						}
						totalFlagDays += flagDays
					}
				}
				if totalFlagDays > 0 {
					toPrint += separator
					toPrint += color.WhiteString("Flag: %d", totalFlagDays)
				}
			}

			if issueDetails.EpicLink != "" {
				toPrint += separator
				toPrint += color.GreenString("Epic: %v", issueDetails.EpicLink)
			}
			if len(issueDetails.Labels) > 0 {
				toPrint += separator
				toPrint += color.BlueString("Labels: %v", strings.Join(issueDetails.Labels, ", "))
			}
			//if issueDetails.Sprint != "" {
			//	toPrint += separator
			//	toPrint += color.GreenString("Sprint: %v", issueDetails.Sprint)
			//}
			if len(issueDetails.CustomFields) > 0 {
				for _, customField := range issueDetails.CustomFields {
					toPrint += separator
					toPrint += customField.String()
				}
			}
			if issueDetails.TransitionDetails != nil {
				toPrint += color.YellowString(" (%s)", issueDetails.TransitionDetails.StatusTo)
			}
			toPrint += "\n"
			_, _ = fmt.Fprintf(color.Output, toPrint)
		}
	}
}

func printAverageByStatus(issueDetails []IssueDetails) {
	totalDurationByStatusMap := make(map[string]time.Duration)
	var totalDuration time.Duration
	for _, issueDetails := range issueDetails {
		for status, duration := range issueDetails.GetDurationByStatus() {
			totalDurationByStatusMap[status] += duration
			totalDuration += duration
		}
	}
	title("\n> Average by Status\n")
	for status, duration := range totalDurationByStatusMap {
		statusPercent := float64(duration*100) / float64(totalDuration)
		fmt.Printf("%v = %.2f%%", status, statusPercent)
		warn(" [%s]\n", durafmt.Parse(duration))
	}
}

func printAverageByStatusType(issueDetails []IssueDetails) {
	var totalDuration time.Duration
	totalDurationByStatusTypeMap := make(map[string]time.Duration)
	for _, issueDetails := range issueDetails {
		for status, duration := range issueDetails.GetDurationByStatus() {
			totalDurationByStatusTypeMap[getIssueTypeByStatus(status)] += duration
			totalDuration += duration
		}
	}
	title("\n> Average by Status Type\n")
	for statusType, duration := range totalDurationByStatusTypeMap {
		statusPercent := float64(duration*100) / float64(totalDuration)
		fmt.Printf("%v = %.2f%%", statusType, statusPercent)
		warn(" [%s]\n", durafmt.Parse(duration))
	}
}

func printWIP(issueDetails []IssueDetails, weekDays int) {
	var wipMonthly int
	var totalWipDuration time.Duration
	for _, issueDetails := range issueDetails {
		totalWipDuration += issueDetails.GetWipAndIdleTotalDuration()
		if issueDetails.GetWipAndIdleTotalDuration().Hours() > 1 {
			wipMonthly++
		}
	}
	title("\n> WIP/Idle\n")
	fmt.Printf("Monthly: ")
	warn("%d tasks were in WIP/Idle\n", wipMonthly)
	//totalWipDays := int(math.Round(totalWipDuration.Hours() / 24))
	//fmt.Printf("Average: ")
	//warn("%d tasks\n", wipMonthly/totalWipDays)
}

func printThroughput(issueDetails []IssueDetails) {
	var totalThroughput int
	throughputMap := make(map[string]int)
	for _, issueDetails := range issueDetails {
		if !issueDetails.ResolvedDate.IsZero() {
			throughputMap[issueDetails.IssueType]++
			totalThroughput++
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
			wipByType += issueDetails.GetWipAndIdleTotalDuration()
		}
		typeThroughput := len(issueDetailsArray)
		throughputMonthly += typeThroughput
		totalWipDuration += wipByType
		leadTimeByTypeMap[issueType] = float64(wipByType) / float64(typeThroughput)
	}

	title("\n> Lead time\n")
	fmt.Printf("Average: ")
	wipDays := getDays(totalWipDuration)
	warn("%d days\n", wipDays/throughputMonthly)
	fmt.Printf("By issue type:\n")
	for issueType, leadTime := range leadTimeByTypeMap {
		fmt.Printf("- %v: ", issueType)
		warn("%d days\n", getDays(time.Duration(leadTime)))
	}
}
