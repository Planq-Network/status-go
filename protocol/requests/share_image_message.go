package requests

import (
	"errors"

	"github.com/status-im/status-go/eth-node/types"
)

var ErrShareMessageInvalidID = errors.New("share-image-message: invalid id")
var ErrShareMessageEmptyUsers = errors.New("share-image-message: empty users")

type ShareImageMessage struct {
	MessageID   string           `json:"id"`
	Users       []types.HexBytes `json:"users"`
	Text        string           `json:"text"`
	ContentType int32            `json:"content-type"`
}

func (s *ShareImageMessage) Validate() error {
	if len(s.MessageID) == 0 {
		return ErrShareMessageInvalidID
	}

	if len(s.Users) == 0 {
		return ErrShareMessageEmptyUsers
	}

	return nil
}
