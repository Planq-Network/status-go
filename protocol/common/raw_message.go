package common

import (
	"crypto/ecdsa"

	"github.com/planq-network/status-go/protocol/protobuf"
)

// RawMessage represent a sent or received message, kept for being able
// to re-send/propagate
type RawMessage struct {
	ID                   string
	LocalChatID          string
	LastSent             uint64
	SendCount            int
	Sent                 bool
	ResendAutomatically  bool
	SkipEncryption       bool
	SendPushNotification bool
	MessageType          protobuf.ApplicationMetadataMessage_Type
	Payload              []byte
	Sender               *ecdsa.PrivateKey
	Recipients           []*ecdsa.PublicKey
	SkipGroupMessageWrap bool
	SendOnPersonalTopic  bool
}
