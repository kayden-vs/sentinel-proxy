#!/usr/bin/env bash

fuser -k 8080/tcp
fuser -k 9090/tcp
fuser -k 9100/tcp