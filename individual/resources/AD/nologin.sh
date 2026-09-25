#!/bin/bash

#Argument 1: host

#Check if ldapsearch is installed
if ! [ -x "$(command -v ldapsearch)" ]; then
  echo 'Error: ldap-utils is not installed.' >&2
  exit 1
fi

if [ $# -ne 1 ] || [ -z "$1" ]; then
  echo "Usage: $0 <host>"
  exit 1
fi

#Anonymous query. Any LDAP answer (success, bind required, no such object...)
#means the directory is up; 255 means the server could not be contacted.
ldapsearch -x -o nettimeout=2 -H ldap://$1 -s base > /dev/null 2>&1

if [ $? -eq 255 ]; then
    echo "No AD found"
    exit 1
fi

exit 0
