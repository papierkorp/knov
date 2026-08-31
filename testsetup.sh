#!/bin/bash

folder="./temp-test-knov"

rm -rf $folder
mkdir $folder
make prod && cp bin/knov* $folder && cd $folder
