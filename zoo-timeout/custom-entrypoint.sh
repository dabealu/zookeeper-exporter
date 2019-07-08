#!/bin/bash

set -e

# Drop traffic to zookeeper
echo 'about to setup iptables to drop packets on 2181'
iptables -A INPUT -p tcp --destination-port 2181 -j DROP
echo 'ip tables rules setup'

# Call the original entrypoint script
echo 'now calling original entrypoint'
echo '----'
