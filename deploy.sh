#!/bin/bash
set -e

cd /home/isucon/isucon14-practice
git pull origin feature/keita_test

sudo systemctl stop isuride-go

cp -a go/*.go go/go.mod go/go.sum /home/isucon/webapp/go/
cp -a sql/. /home/isucon/webapp/sql/

cd /home/isucon/webapp/go
go build -o isuride

sudo systemctl start isuride-go
echo "deploy done"