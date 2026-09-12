#!/bin/bash
set -e

cd /home/isucon/isucon14-practice
git pull origin feature/keita_test

sudo systemctl stop isuride-python

cp -a python/. /home/isucon/webapp/python/

cd /home/isucon/webapp/python
# 依存関係を足したらコメントを外す
# ~/.local/bin/uv sync

sudo systemctl start isuride-python
echo "deploy done"
