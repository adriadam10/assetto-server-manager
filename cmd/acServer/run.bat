go build -a
copy acServer.exe assetto/acServer.exe
copy config.yml assetto/config.yml
pushd assetto
acServer.exe
popd
