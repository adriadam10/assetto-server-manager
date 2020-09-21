#!/bin/bash
set -xe
go build -a
cp config.yml assetto/config.yml
cp acServer assetto/acServer
pushd assetto
	./acServer
popd
