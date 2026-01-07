test:
	go test -coverpkg=./... -coverprofile .coverage.txt -covermode=set -failfast ./...
	go tool cover -func .coverage.txt
	go tool cover -html=.coverage.txt -o coverage.html