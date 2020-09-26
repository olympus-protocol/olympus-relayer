#!/bin/bash

function configure_systemd() {

sudo rm -rf /etc/systemd/system/olympus-relayer.service

cat << EOF > /etc/systemd/system/olympus-relayer.service
[Unit]
Description=Olympus Relayer
After=network.target

[Service]
Type=simple
User=root
LimitNOFILE=1024

Restart=on-failure
RestartSec=10

ExecStart=/usr/local/bin/olympus-relayer --datadir=/opt/olympus-relayer --logfile
 WorkingDirectory=/opt/olympus-relayer

PermissionsStartOnly=true

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  sleep 3
}

echo "Building and Installing Olympus Relayer"

go build ./ &> /dev/null

cp ./olympus-relayer /usr/local/bin/

mkdir mkdir -p /opt/olympus-relayer

configure_systemd

title="Olympus Relayer Installed"
instructions_first="The program is installed in the systemd services"
instructions_second="To start the program run 'service olympus-relayer start'"
instructions_third="The net_key.dat, peerstore and logs are in /opt/olympus-relayer"
instructions_fourth="Check the logs to retrieve the connection string"
log_check="tail -f /opt/olympus-relayer/logger.log"


printf %"$(tput cols)"s |tr " " "*"
printf %"$(tput cols)"s |tr " " " "
printf "%*s\n" $(((${#title}+$(tput cols))/2)) "$title"
printf "%*s\n" $(((${#instructions_first}+$(tput cols))/2)) "$instructions_first"
printf "%*s\n" $(((${#instructions_second}+$(tput cols))/2)) "$instructions_second"
printf "%*s\n" $(((${#instructions_third}+$(tput cols))/2)) "$instructions_third"
printf "%*s\n" $(((${#instructions_fourth}+$(tput cols))/2)) "$instructions_fourth"
printf "%*s\n" $(((${#log_check}+$(tput cols))/2)) "$log_check"

printf %"$(tput cols)"s |tr " " " "
printf %"$(tput cols)"s |tr " " "*"
