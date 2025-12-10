module github.com/tidepool-org/clinic/redox_models

go 1.25.5

require go.mongodb.org/mongo-driver v1.13.1

require github.com/google/go-cmp v0.6.0 // indirect

replace (
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d => golang.org/x/crypto v0.18.0 // Resolve GO-2023-2402
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b => golang.org/x/net v0.20.0 // Resolve GO-2023-2102, GO-2023-1988, GO-2023-1571, GO-2023-1495, GO-2022-1144, GO-2022-0969
	golang.org/x/net v0.10.0 => golang.org/x/net v0.20.0 // Resolve GO-2023-2102, GO-2023-1988
)
