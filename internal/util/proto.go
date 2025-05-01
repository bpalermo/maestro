package util

import (
	"buf.build/go/protoyaml"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func MustMarshalProtoToYaml(message proto.Message) []byte {
	options := protoyaml.MarshalOptions{
		Indent: 2,
	}
	b, err := options.Marshal(message)
	if err != nil {
		panic(err)
	}
	return b
}

func MustAny(message proto.Message) *anypb.Any {
	pb, err := anypb.New(message)
	if err != nil {
		panic(err)
	}
	return pb
}
