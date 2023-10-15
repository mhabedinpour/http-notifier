set -e
go test $(go list ./... | grep -v /vendor/) -v -race -coverprofile cover.out
go tool cover -func=cover.out | grep total