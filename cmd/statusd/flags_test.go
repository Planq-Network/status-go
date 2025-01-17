package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/planq-network/status-go/params"
)

// nolint: deadcode
func TestStatusFlag(t *testing.T) {
	service := "status"

	scenarios := []struct {
		ipcEnabled  bool
		httpEnabled bool
		flag        string
		err         error
		enabled     bool
		public      bool
	}{
		// no flags
		{},
		// -status=ipc -ipc
		{
			ipcEnabled: true,
			flag:       "ipc",
			enabled:    true,
		},
		// -status=http -http
		{
			httpEnabled: true,
			flag:        "http",
			enabled:     true,
			public:      true,
		},
		// -status=ipc -http -ipc
		{
			httpEnabled: true,
			ipcEnabled:  true,
			flag:        "ipc",
			enabled:     true,
		},
		// -http -ipc
		{
			httpEnabled: true,
			ipcEnabled:  true,
			flag:        "",
		},
		// -status=ipc
		{
			err:  errStatusServiceRequiresIPC,
			flag: "ipc",
		},
		// -status=http
		{
			err:  errStatusServiceRequiresHTTP,
			flag: "http",
		},
		// -status=bad-value
		{
			err:  errStatusServiceInvalidFlag,
			flag: "bad-value",
		},
	}

	for i, s := range scenarios {
		msg := fmt.Sprintf("scenario %d", i)

		c, err := params.NewNodeConfig("", 0)
		require.Nil(t, err, msg)

		c.IPCEnabled = s.ipcEnabled
		c.HTTPEnabled = s.httpEnabled

		c, err = configureStatusService(s.flag, c)

		if s.err != nil {
			require.Equal(t, s.err, err, msg)
			require.Nil(t, c, msg)
			continue
		}

		require.Nil(t, err, msg)
		require.Equal(t, s.enabled, c.EnableStatusService, msg)

		modules := c.FormatAPIModules()
		if s.public {
			require.Contains(t, modules, service, msg)
		} else {
			require.NotContains(t, modules, service, msg)
		}
	}
}
