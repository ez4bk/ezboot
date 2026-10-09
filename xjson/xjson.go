package xjson

import (
	"encoding/json"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/proto"
)

// Convert 使用JSON将数据从src转换到dst中
func Convert(dst any, src any) error {
	if src == nil || dst == nil {
		return nil
	}

	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// MustMarshal 将v以JSON形式进行编码并返回
func MustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// MustMarshalPb 将v以JSON形式进行编码并返回
func MustMarshalPb(v proto.Message) []byte {
	var m runtime.JSONPb
	data, err := m.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
