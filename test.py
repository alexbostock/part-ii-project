import os

for i in range(2, 11):
    x = 2**i

    print(x)

    os.system("go run main.go -n=1000 -rate=100 -t=100 -seed=" + str(x) + " | python3 transactionStats.py")
    os.system("go run main.go -n=1000 -rate=1000 -t=100 -seed=" + str(x) + " | python3 transactionStats.py")
    os.system("go run main.go -n=1000 -rate=1000 -t=1000 -seed=" + str(x) + " | python3 transactionStats.py")

    print()
