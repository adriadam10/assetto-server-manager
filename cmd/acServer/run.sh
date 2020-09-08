#!/bin/bash
set -xe
go build -a
cp config.yml assetto/config.yml
pushd assetto
	./acServer
popd
