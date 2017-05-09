#!/bin/bash
 
linux_build()
{
  rm -f jira-kanban-metrics 2> /dev/null
  echo "Building for Linux"
  go build -o jira-kanban-metrics *.go
}

windows_build()
{
  rm -f jira-kanban-metrics.exe 2> /dev/null
  echo "Building for Windows"
  GOOS=windows GOARCH=386 go build -o jira-kanban-metrics.exe *.go
}


if [ $# -ne 1 ] || ! [[ $1 =~ ^(linux|windows|all)$ ]];
then
  echo "$0 {linux | windows | all}"
  exit 0
fi

case "$1" in
  "linux") linux_build ;;
  "windows") windows_build ;;
  "all") linux_build; windows_build ;;
esac
