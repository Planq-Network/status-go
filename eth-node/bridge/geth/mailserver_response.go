package gethbridge

import (
	"github.com/planq-network/status-go/eth-node/types"
	"github.com/planq-network/status-go/waku"
	"github.com/planq-network/status-go/wakuv2"
)

// NewWakuMailServerResponseWrapper returns a types.MailServerResponse object that mimics Geth's MailServerResponse
func NewWakuMailServerResponseWrapper(mailServerResponse *waku.MailServerResponse) *types.MailServerResponse {
	if mailServerResponse == nil {
		panic("mailServerResponse should not be nil")
	}

	return &types.MailServerResponse{
		LastEnvelopeHash: types.Hash(mailServerResponse.LastEnvelopeHash),
		Cursor:           mailServerResponse.Cursor,
		Error:            mailServerResponse.Error,
	}
}

// NewWakuV2MailServerResponseWrapper returns a types.MailServerResponse object that mimics Geth's MailServerResponse
func NewWakuV2MailServerResponseWrapper(mailServerResponse *wakuv2.MailServerResponse) *types.MailServerResponse {
	if mailServerResponse == nil {
		panic("mailServerResponse should not be nil")
	}

	return &types.MailServerResponse{
		LastEnvelopeHash: types.Hash(mailServerResponse.LastEnvelopeHash),
		Cursor:           mailServerResponse.Cursor,
		Error:            mailServerResponse.Error,
	}
}
