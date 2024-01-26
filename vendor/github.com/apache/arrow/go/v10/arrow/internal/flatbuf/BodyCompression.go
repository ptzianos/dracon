// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package flatbuf

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

/// Optional compression for the memory buffers constituting IPC message
/// bodies. Intended for use with RecordBatch but could be used for other
/// message types
type BodyCompression struct {
	_tab flatbuffers.Table
}

func GetRootAsBodyCompression(buf []byte, offset flatbuffers.UOffsetT) *BodyCompression {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &BodyCompression{}
	x.Init(buf, n+offset)
	return x
}

func (rcv *BodyCompression) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *BodyCompression) Table() flatbuffers.Table {
	return rcv._tab
}

/// Compressor library.
/// For LZ4_FRAME, each compressed buffer must consist of a single frame.
func (rcv *BodyCompression) Codec() CompressionType {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return CompressionType(rcv._tab.GetInt8(o + rcv._tab.Pos))
	}
	return 0
}

/// Compressor library.
/// For LZ4_FRAME, each compressed buffer must consist of a single frame.
func (rcv *BodyCompression) MutateCodec(n CompressionType) bool {
	return rcv._tab.MutateInt8Slot(4, int8(n))
}

/// Indicates the way the record batch body was compressed
func (rcv *BodyCompression) Method() BodyCompressionMethod {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return BodyCompressionMethod(rcv._tab.GetInt8(o + rcv._tab.Pos))
	}
	return 0
}

/// Indicates the way the record batch body was compressed
func (rcv *BodyCompression) MutateMethod(n BodyCompressionMethod) bool {
	return rcv._tab.MutateInt8Slot(6, int8(n))
}

func BodyCompressionStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func BodyCompressionAddCodec(builder *flatbuffers.Builder, codec CompressionType) {
	builder.PrependInt8Slot(0, int8(codec), 0)
}
func BodyCompressionAddMethod(builder *flatbuffers.Builder, method BodyCompressionMethod) {
	builder.PrependInt8Slot(1, int8(method), 0)
}
func BodyCompressionEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
