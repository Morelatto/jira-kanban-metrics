package main

import (
	"fmt"
	"github.com/hako/durafmt"
	"time"
)

var CLParameters struct {
	StartDate string `docopt:"<start>"`
	EndDate   string `docopt:"<end>"`
	Jql       string `docopt:"<JQL>"`
	Debug     bool
}

var BoardCfg struct {
	JiraUrl    string
	Login      string
	Password   string
	Project    string
	OpenStatus []string
	WipStatus  []string
	IdleStatus []string
	DoneStatus []string
}

type IssueDetails struct {
	Key               string
	Title             string
	IssueType         string
	CreatedDate       time.Time
	WipDate           time.Time
	ResolvedDate      time.Time
	EpicLink          string
	Sprint            string
	Labels            []string
	CustomFields      []CustomField
	TransitionDetails *TransitionDetails
	FlagDetails       []FlagDetails
	Description       string
}

type FlagDetails struct {
	FlagStart time.Time
	FlagEnd   time.Time
}

func (f *FlagDetails) GetFlagDuration() time.Duration {
	if f.FlagEnd.IsZero() {
		f.FlagEnd = parseDate(CLParameters.EndDate)
	}
	return getTransitionDuration(f.FlagStart, f.FlagEnd)
}

func (i *IssueDetails) GetWipAndIdleTotalDuration() time.Duration {
	var wipIdleTotal time.Duration
	var wipIdleStatus = append(BoardCfg.WipStatus, BoardCfg.IdleStatus...)
	var currentTransition = i.TransitionDetails
	for {
		if containsStatus(wipIdleStatus, currentTransition.StatusFrom) {
			wipIdleTotal += currentTransition.getTotalDuration()
		}
		if currentTransition.PreviousTransition == nil {
			break
		}
		currentTransition = currentTransition.PreviousTransition
	}
	return wipIdleTotal
}

func (i *IssueDetails) GetWipTotalDuration() time.Duration {
	var wipTotal time.Duration
	var currentTransition = i.TransitionDetails
	for {
		if containsStatus(BoardCfg.WipStatus, currentTransition.StatusFrom) {
			wipTotal += currentTransition.getTotalDuration()
		}
		if currentTransition.PreviousTransition == nil {
			break
		}
		currentTransition = currentTransition.PreviousTransition
	}
	return wipTotal
}

func (i *IssueDetails) GetDurationByStatus() map[string]time.Duration {
	statusMap := make(map[string]time.Duration)
	var currentTransition = i.TransitionDetails
	for {
		if currentTransition.StatusFrom != "" {
			statusMap[currentTransition.StatusFrom] += currentTransition.getTotalDuration()
		}
		if currentTransition.PreviousTransition == nil {
			break
		}
		currentTransition = currentTransition.PreviousTransition
	}
	return statusMap
}

type TransitionDetails struct {
	Timestamp          time.Time
	StatusFrom         string
	StatusTo           string
	PreviousTransition *TransitionDetails
}

// Calculates time difference between transitions subtracting weekend days
func (t *TransitionDetails) getTotalDuration() time.Duration {
	return getTransitionDuration(t.PreviousTransition.Timestamp, t.Timestamp)
}

func getTransitionDuration(firstTransition time.Time, secondTransition time.Time) time.Duration {
	transitionDuration := secondTransition.Sub(firstTransition)
	weekendDays := countWeekendDays(firstTransition, secondTransition)
	if weekendDays > 0 {
		if getDays(transitionDuration) >= weekendDays {
			transitionDuration -= time.Duration(weekendDays) * time.Hour * 24
		} else {
			transitionDuration = 0
		}
	}
	return transitionDuration
}

func (t *TransitionDetails) PrintFrom() {
	info("%s %s\n", formatBrDateWithTime(t.PreviousTransition.Timestamp), t.StatusFrom)
}

func (t *TransitionDetails) PrintTo() {
	info("%s %s", formatBrDateWithTime(t.Timestamp), t.StatusTo)
	if t.PreviousTransition != nil {
		warn(" [%s]", durafmt.Parse(t.getTotalDuration()))
	}
	fmt.Println()
}
