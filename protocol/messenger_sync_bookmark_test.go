package protocol

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"testing"

	gethbridge "github.com/planq-network/status-go/eth-node/bridge/geth"
	"github.com/planq-network/status-go/eth-node/crypto"
	"github.com/planq-network/status-go/protocol/encryption/multidevice"
	"github.com/planq-network/status-go/protocol/tt"
	"github.com/planq-network/status-go/services/browsers"
	"github.com/planq-network/status-go/waku"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/planq-network/status-go/eth-node/types"
)

func TestMessengerSyncBookmarkSuite(t *testing.T) {
	suite.Run(t, new(MessengerSyncBookmarkSuite))
}

type MessengerSyncBookmarkSuite struct {
	suite.Suite
	m          *Messenger        // main instance of Messenger
	privateKey *ecdsa.PrivateKey // private key for the main instance of Messenger

	// If one wants to send messages between different instances of Messenger,
	// a single Waku service should be shared.
	shh types.Waku

	logger *zap.Logger
}

func (s *MessengerSyncBookmarkSuite) SetupTest() {
	s.logger = tt.MustCreateTestLogger()

	config := waku.DefaultConfig
	config.MinimumAcceptedPoW = 0
	shh := waku.New(&config, s.logger)
	s.shh = gethbridge.NewGethWakuWrapper(shh)
	s.Require().NoError(shh.Start())

	s.m = s.newMessenger(s.shh)
	s.privateKey = s.m.identity
	// We start the messenger in order to receive installations
	_, err := s.m.Start()
	s.Require().NoError(err)
}

func (s *MessengerSyncBookmarkSuite) TearDownTest() {
	s.Require().NoError(s.m.Shutdown())
}

func (s *MessengerSyncBookmarkSuite) newMessenger(shh types.Waku) *Messenger {
	privateKey, err := crypto.GenerateKey()
	s.Require().NoError(err)

	messenger, err := newMessengerWithKey(s.shh, privateKey, s.logger, nil)
	s.Require().NoError(err)

	return messenger
}

func (s *MessengerSyncBookmarkSuite) TestSyncBookmark() {
	//add bookmark
	bookmark := browsers.Bookmark{
		Name:    "status official site",
		URL:     "https://status.im",
		Removed: false,
	}
	_, err := s.m.browserDatabase.StoreBookmark(bookmark)
	s.Require().NoError(err)

	// pair
	theirMessenger, err := newMessengerWithKey(s.shh, s.privateKey, s.logger, nil)
	s.Require().NoError(err)

	err = theirMessenger.SetInstallationMetadata(theirMessenger.installationID, &multidevice.InstallationMetadata{
		Name:       "their-name",
		DeviceType: "their-device-type",
	})
	s.Require().NoError(err)
	response, err := theirMessenger.SendPairInstallation(context.Background())
	s.Require().NoError(err)
	s.Require().NotNil(response)
	s.Require().Len(response.Chats(), 1)
	s.Require().False(response.Chats()[0].Active)

	// Wait for the message to reach its destination
	response, err = WaitOnMessengerResponse(
		s.m,
		func(r *MessengerResponse) bool { return len(r.Installations) > 0 },
		"installation not received",
	)

	s.Require().NoError(err)
	actualInstallation := response.Installations[0]
	s.Require().Equal(theirMessenger.installationID, actualInstallation.ID)
	s.Require().NotNil(actualInstallation.InstallationMetadata)
	s.Require().Equal("their-name", actualInstallation.InstallationMetadata.Name)
	s.Require().Equal("their-device-type", actualInstallation.InstallationMetadata.DeviceType)

	err = s.m.EnableInstallation(theirMessenger.installationID)
	s.Require().NoError(err)

	// sync
	err = s.m.SyncBookmark(context.Background(), &bookmark)
	s.Require().NoError(err)

	// Wait for the message to reach its destination
	err = tt.RetryWithBackOff(func() error {
		response, err = theirMessenger.RetrieveAll()
		if err != nil {
			return err
		}
		if response.Bookmarks != nil {
			return nil
		}
		return errors.New("Not received all bookmarks")
	})

	s.Require().NoError(err)

	bookmarks, err := theirMessenger.browserDatabase.GetBookmarks()
	s.Require().NoError(err)
	s.Require().Equal(1, len(bookmarks))
	s.Require().False(bookmarks[0].Removed)

	// sync removed state
	bookmark.Removed = true
	err = s.m.SyncBookmark(context.Background(), &bookmark)
	s.Require().NoError(err)

	// Wait for the message to reach its destination
	err = tt.RetryWithBackOff(func() error {
		response, err = theirMessenger.RetrieveAll()
		if err != nil {
			return err
		}
		if response.Bookmarks != nil {
			return nil
		}
		return errors.New("Not received all bookmarks")
	})
	bookmarks, err = theirMessenger.browserDatabase.GetBookmarks()
	s.Require().NoError(err)
	s.Require().Equal(1, len(bookmarks))
	s.Require().True(bookmarks[0].Removed)

	s.Require().NoError(theirMessenger.Shutdown())

}
