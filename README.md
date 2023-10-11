# gonsec

Reads in a list of domains and searches for additional domains from NSEC records.

## Usage

`echo example.com | gonsec`

or

`cat domains.txt | gonsec`

or 

`gonsec example.com`

## Install

You need to have the latest version (1.19+) of Go installed and configured (i.e. with $GOPATH/bin in your $PATH):


`go install github.com/cybercdh/gonsec@latest`