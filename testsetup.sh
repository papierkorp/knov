#!/bin/bash

folder="./temp-knov"

rm -rf $folder
mkdir $folder
make prod && cp bin/knov* $folder && cp .env.example $folder/.env && cd $folder
