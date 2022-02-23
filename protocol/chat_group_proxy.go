package protocol

import (
	v1protocol "github.com/planq-network/status-go/protocol/v1"
)

func newProtocolGroupFromChat(chat *Chat) (*v1protocol.Group, error) {
	return v1protocol.NewGroupWithEvents(chat.ID, chat.MembershipUpdates)
}
