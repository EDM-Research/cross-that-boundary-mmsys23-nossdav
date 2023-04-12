module github.com/uccmisl/godash

go 1.16

require (
	github.com/cavaliercoder/grab v2.0.1-0.20200331080741-9f014744ee41+incompatible
	github.com/francoispqt/gojay v1.2.13
	github.com/golang/protobuf v1.5.2
	github.com/hashicorp/consul/api v1.4.0
	github.com/lucas-clemente/quic-go v0.23.0
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b
	gonum.org/v1/gonum v0.7.0
	google.golang.org/grpc v1.19.0
)

replace github.com/lucas-clemente/quic-go => ../quic-go
