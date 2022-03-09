#!/bin/sh
go build -o /bin/hello /app/hello.go
chmod 777 /bin/hello
/bin/hello