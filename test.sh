go run main.go | tee >(python3 transactionStats.py) | python3 consistencyTest.py
