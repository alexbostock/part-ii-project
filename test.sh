go test github.com/alexbostock/part-ii-project/simnet/datastore

echo

go run main.go | tee >(python3 transactionStats.py) | python3 consistencyTest.py
