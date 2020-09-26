#!/bin/bash

function configure_systemd() {

sudo rm -rf /etc/systemd/system/olympus_relayer.service

cat << EOF > /etc/systemd/system/olympus_relayer.service
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

    ExecStart=/usr/local/bin/relayer

    PermissionsStartOnly=true
    PermissionsStartOnly=true
    ExecStartPre=/bin/mkdir -p /var/log/relayer
    ExecStartPre=/bin/chown syslog:adm /var/log/relayer
    ExecStartPre=/bin/chmod 755 /var/log/relayer
    StandardOutput=syslog
    StandardError=syslog
    SyslogIdentifier=relayer

    [Install]
    WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  sleep 3
}

echo "Building and Installing Olympus Relayer"

go build -o relayer ./ &> /dev/null

cp ./relayer /usr/local/bin/

configure_systemd
