#!/bin/bash -e

VALUE=$( awk -F '=' '/leaderServes/ {print $2}' zoo-conf/zoo.cfg.1 )

if [ "$VALUE" == 'no' ]; then
    VALUE='yes'
else
    VALUE='no'
fi

for I in {1..3}; do
    sed -i '' "s/leaderServes=.*$/leaderServes=${VALUE}/" zoo-conf/zoo.cfg.${I}
done

echo "leaderServes changed to '${VALUE}'"
