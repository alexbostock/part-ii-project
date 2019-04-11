go test github.com/alexbostock/part-ii-project/net
go test github.com/alexbostock/part-ii-project/datastore

echo

go run main.go -t=100 > output.log
cat output.log | python3 scripts/parse-output.py
