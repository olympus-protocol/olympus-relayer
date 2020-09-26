#!/bin/bash


arch=$(uname -i)

if [ "$arch" == 'x86_64' ];
then
export GO=go1.14.4.linux-amd64.tar.gz
fi

if [ "$arch" == 'aarch64' ];
then
export GO=go1.14.4.linux-arm64.tar.gz
fi

echo "Checking dependencies"

if ! command -v git version &> /dev/null
then
    echo "Git not installed, downloading..."
    apt install git
    exit
fi

if ! command -v go version &> /dev/null
then
    echo "Golang not installed, downloading..."
    cd /opt && curl https://storage.googleapis.com/golang/${GO} -o ${GO}
    tar zxf ${GO} && rm ${GO}
    ln -s /opt/go/bin/go /usr/bin/
    export GOPATH=/root/go
    exit
fi

echo "Downloading Olympus Relayer"

git clone https://github.com/olympus-protocol/olympus-relayer

bash ./build.sh


