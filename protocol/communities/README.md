# Communities encryption
protocol.Messenger.CreateCommunity invokes communities.Manager.CreateCommunity
Parameter is requests.CreateCommunity
Then we init some filters.

protocol.Messenger.dispatchMessage() with protocol.common.MessageSender's SendPrivate, SendGroup, SendCommunityMessage
protocol.Messenger.handleRetrievedMessages() with a switch-case, invokes MessageSender.HandleMessages()

protocol.common.MessageSender, SendPrivate, SendGroup, SendCommunityMessage


## Regenerate protobuf
protoc --gofast_out=paths=source_relative:. application_metadata_message.proto


