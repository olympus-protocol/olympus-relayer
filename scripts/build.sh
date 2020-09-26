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

    ExecStart=/usr/local/bin/olympus-relayer --datadir=/opt/olympus-relayer
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
