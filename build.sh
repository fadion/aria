#!/usr/bin/env bash

set -e

mkdir -p release
cd release

echo "\033[0;31mCreated /release directory.\033[0m"

build() {
    os=$1
    arch=$2

    if [ ${os} = darwin ]; then
        buildos="macos"
    else
        buildos=${os}
    fi

    if [ ${arch} = amd64 ]; then
        buildarch="64bit"
    else
        buildarch="32bit"
    fi

    binary=aria
    release="$binary-$buildos-$buildarch"

    if [ ${os} = windows ]; then
        binary="$binary.exe"
    fi

    env GOOS=${os} GOARCH=${arch} go build -v -o ${binary} ../aria.go

    if [ ${os} = linux ]; then
        tar czf "$release.tar.gz" "$binary"
    else
        zip "$release.zip" "$binary"
    fi

    rm -f ${binary}

    echo "\033[0;31mCreated release for '$buildos' on '$buildarch'.\033[0m"
}

# MacOS
build darwin amd64
build darwin 386

# Linux
build linux amd64
build linux 386

# Windows
build windows amd64
build windows 386