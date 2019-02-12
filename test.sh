go test github.com/alexbostock/part-ii-project/net
go test github.com/alexbostock/part-ii-project/datastore

echo

go run main.go -t=1000 > output.log
cat output.log | python3 parseOutput.py
