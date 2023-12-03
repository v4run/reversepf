package commands

import (
	"encoding/json"
	"io"
)

type CommandType int8

const (
	TypeInit CommandType = iota
)

type Command struct {
	Type CommandType `json:"type"`
}

func (c Command) Bytes() []byte {
	data, _ := json.Marshal(c)
	return data
}

func (c Command) String() string {
	return string(c.Bytes())
}

func ReadCommand(reader io.Reader) (Command, error) {
	var cmd Command
	if err := json.NewDecoder(reader).Decode(&cmd); err != nil {
		return Command{}, err
	}
	return cmd, nil
}
