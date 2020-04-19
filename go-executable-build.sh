#!/usr/bin/env bash

# Source: https://www.digitalocean.com/community/tutorials/how-to-build-go-executables-for-multiple-platforms-on-ubuntu-16-04

platforms=("darwin" "linux")

for platform in "${platforms[@]}"
do
    #platform=`sed'/,//'$platform`
    output_name="kubediscovery-"$platform
    if [ $platform = "windows" ]; then
        output_name+='.exe'
    fi

    env GOOS=$GOOS go build -o $output_name .
    if [ $? -ne 0 ]; then
        echo 'An error has occurred! Aborting the script execution...'
        exit 1
    fi
done