#/bin/bash

rm -rf ../test-knov
mkdir ../test-knov
make prod
cp bin/knov* ../test-knov/
cd ../test-knov/
