package api

//go:generate mockgen -source=./client.go -destination=./mock.go -package api ClientInterface
//go:generate mockgen -source=./client.go -destination=./mock.go -package api ClientWithResponsesInterface
