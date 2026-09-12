#!/bin/bash
set -e

echo "=== Deploying to AWS Server ==="

ssh isucon-practice << 'EOF'
  # isucon ユーザーとしてコマンドを実行する
  sudo -i -u isucon bash << 'INNER_EOF'
    set -e
    
    # 1. webappへ移動して最新コードを取得
    cd /home/isucon/webapp
    git pull origin main

    # 2. Goのパスを通してからビルド
    export PATH=$PATH:/usr/local/go/bin:/home/isucon/local/go/bin
    cd /home/isucon/webapp/go
    go build -o isuride
INNER_EOF

  # 3. sudo権限でサービスを再起動
  sudo systemctl restart isuride-go.service
EOF

echo "=== Deployment Finished! ==="
