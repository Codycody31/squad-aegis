// Original source: https://github.com/SquadGO/squad-rcon-go

package utils

import (
	"bytes"
	"encoding/binary"
)

type RconResponse struct {
	Size int32
	ID   int32
	Type int32
	Body string
}

func Encode(typeID int, id int, body string) []byte {
	size := int32(len([]byte(body)) + 14)
	buf := make([]byte, size)

	binary.LittleEndian.PutUint32(buf[0:4], uint32(size-4))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(id))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(typeID))
	copy(buf[12:size-2], body)
	binary.LittleEndian.PutUint16(buf[size-2:size], 0)

	return buf
}

func Decode(buffer []byte) RconResponse {
	var response RconResponse

	response.Size = int32(binary.LittleEndian.Uint32(buffer[0:4]))
	response.ID = int32(binary.LittleEndian.Uint32(buffer[4:8]))
	response.Type = int32(binary.LittleEndian.Uint32(buffer[8:12]))
	response.Body = string(bytes.Trim(buffer[12:len(buffer)-2], "\x00"))

	return response
}
