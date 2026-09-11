#!/bin/bash
sudo alp ltsv --file /var/log/nginx/access.log \
  --uri-label=uri \
  --method-label=method \
  --status-label=status \
  --reqtime-label=reqtime \
  --size-label=size \
  --sort sum -r \
  -m "/api/app/rides/.+/evaluation,/api/chair/rides/.+/status,/api/internal/matching"
