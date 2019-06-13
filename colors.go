package main

import "github.com/zchee/color"

var title = color.New(color.Bold, color.FgBlue).PrintfFunc()
var info = color.New(color.Bold, color.FgYellow).PrintfFunc()
var warn = color.New(color.Bold, color.FgRed).PrintfFunc()
