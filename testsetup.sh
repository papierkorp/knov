#!/bin/bash

folder="./temp-knov"

rm -rf $folder
mkdir $folder
make prod && cp bin/knov* $folder && cd $folder
