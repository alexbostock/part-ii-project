go test github.com/alexbostock/part-ii-project/simnet
go test github.com/alexbostock/part-ii-project/simnet/datastore
go test github.com/alexbostock/part-ii-project/simnet/quorumlock

mkdir data

echo

go run main.go | tee >(python3 transactionStats.py) | python3 consistencyTest.py
