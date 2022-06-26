#!/bin/bash
proto_path=$(dirname "$0")
cd "${proto_path}" || exit
docker build --platform linux/amd64 -t local/go-template-protos .
docker run --rm -it --platform linux/amd64 -v "$(pwd)":/workdir local/go-template-protos bash -c "buf lint"
rm -rf gen
docker run --rm -it --platform linux/amd64 -v "$(pwd)":/workdir local/go-template-protos bash -c "buf generate"
