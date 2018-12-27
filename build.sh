#!/usr/bin/env bash

GOOS=linux GOARCH=amd64 go build -o TimeCard
zip TimeCard.zip TimeCard