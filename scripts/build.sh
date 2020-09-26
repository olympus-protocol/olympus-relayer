#!/bin/bash

function configure_systemd() {

sudo rm -r /etc/systemd/system/olympus_relayer.service

cat << EOF > /etc/systemd/system/olympus_relayer.service
    [Unit]
    Description=Olympus Relayer
    After=network.target

    [Service]
    ExecStart=/usr/local/bin/olympus_relayer
    Type=simple
    User=root
    Restart=on-failure
    TimeoutStopSec=300
    LimitNOFILE=500000
    PrivateTmp=true
    ProtectSystem=full
    NoNewPrivileges=true
    PrivateDevices=true
    StandardOutput=append:/var/log/olympus_relayer.log
    [Install]
    WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  sleep 3
}

echo "Building and Installing Olympus Relayer"

cd ./olympus-relayer || exit

go build ./

cp ./olympus-relayer /usr/local/bin/

configure_systemd
