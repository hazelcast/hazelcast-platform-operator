module github.com/hazelcast/hazelcast-platform-operator/maploader

go 1.16

require (
	github.com/hazelcast/hazelcast-go-client v1.2.0
	golang.org/x/tools v0.1.7 // indirect
	k8s.io/client-go v0.20.2
)

// to fix vulnerability: CVE-2021-3121 in github.com/gogo/protobuf < v1.3.2
replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2