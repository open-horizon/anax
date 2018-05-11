#!/bin/bash

# Modify the anax config for the wiotp prod environment

# Check the exit status of the previously run command and exit if nonzero
checkrc() {
  if [[ $1 -ne 0 ]]; then
  	if [[ -n "$2" ]]; then
  		fromStr="from: $2"
  	else
  		fromStr="from the last command"
  	fi
    echo "Error: exit code $1 $fromStr"
    exit $1
  fi
}

# Nothing special needs to be done in the wiotp prod environment right now, so this is just a placeholder for future needs