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
    startLimitIntervalSec=60

    ExecStart=/usr/local/bin/olympus-relayer

    PermissionsStartOnly=true
    PermissionsStartOnly=true
    ExecStartPre=/bin/mkdir -p /var/log/olympus-relayer
    ExecStartPre=/bin/chown syslog:adm /var/log/olympus-relayer
    ExecStartPre=/bin/chmod 755 /var/log/olympus-relayer
    StandardOutput=syslog
    StandardError=syslog

    SyslogIdentifier=olympus-relayer

    [Install]
    WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  sleep 3
}

echo "Building and Installing Olympus Relayer"

go build go build ./ &> /dev/null

cp ./olympus-relayer /usr/local/bin/

configure_systemd
