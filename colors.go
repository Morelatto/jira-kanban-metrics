package main

import (
	"github.com/zchee/color"
)

var title = color.New(color.Bold, color.FgBlue).PrintfFunc()
var titleLn = color.New(color.Bold, color.FgBlue).PrintlnFunc()
var info = color.New(color.Bold, color.FgYellow).PrintfFunc()
var infoLn = color.New(color.Bold, color.FgYellow).PrintlnFunc()
var warn = color.New(color.Bold, color.FgRed).PrintfFunc()
var debug = color.New(color.Bold, color.FgGreen).PrintlnFunc()

func Debug(msg string) {
	if CLParameters.Debug {
		debug(msg)
	}
}
