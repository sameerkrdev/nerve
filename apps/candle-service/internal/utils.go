package internal

import (
	"hash/fnv"
)

type APIReponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   string      `json:"error,omitempty"`
}

func Hash(str string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(str))

	return h.Sum32()
}
