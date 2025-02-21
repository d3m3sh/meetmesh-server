#!/bin/bash
tag=$1

[ ! -z "$tag" ] && docker build -t meetmesh-server:$tag . && exit 1

docker build -t meetmesh-server .
