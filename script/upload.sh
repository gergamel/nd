#!/bin/bash
FILE=$1
HOST=https://127.0.0.1:8080
echo "Calulating SHA256..."
SHA256=`sha256sum $1 | tr " " "\n" | head -1`
URL="$HOST/objects/$SHA256"
echo "Uploading $FILE -> $URL"
curl -s -X PUT -F "file=@$FILE" $URL | jq
