// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"context"
	"net"
	"testing"

	"github.com/mattermost/calls-offloader/public"
	"github.com/mattermost/calls-offloader/service/auth"
	"github.com/mattermost/calls-offloader/service/random"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		c, err := public.NewClient(public.ClientConfig{})
		require.Error(t, err)
		require.Equal(t, "failed to parse config: invalid URL value: should not be empty", err.Error())
		require.Nil(t, c)
	})

	t.Run("invalid url", func(t *testing.T) {
		c, err := public.NewClient(public.ClientConfig{URL: "not_a_url"})
		require.Error(t, err)
		require.Equal(t, "failed to parse config: invalid url host: should not be empty", err.Error())
		require.Nil(t, c)
	})

	t.Run("invalid scheme", func(t *testing.T) {
		c, err := public.NewClient(public.ClientConfig{URL: "ftp://invalid"})
		require.Error(t, err)
		require.Equal(t, `failed to parse config: invalid url scheme: "ftp" is not valid`, err.Error())
		require.Nil(t, c)
	})

	t.Run("success http scheme", func(t *testing.T) {
		apiURL := "http://localhost"
		c, err := public.NewClient(public.ClientConfig{URL: apiURL})
		require.NoError(t, err)
		require.NotNil(t, c)
		require.NotEmpty(t, c)
		require.Equal(t, apiURL, c.URL())
	})

	t.Run("success https scheme", func(t *testing.T) {
		apiURL := "https://localhost"
		c, err := public.NewClient(public.ClientConfig{URL: apiURL})
		require.NoError(t, err)
		require.NotNil(t, c)
		require.NotEmpty(t, c)
		require.Equal(t, apiURL, c.URL())
	})

	t.Run("custom dialing function", func(t *testing.T) {
		var called bool
		dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
			called = true
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}

		apiURL := "http://localhost"
		c, err := public.NewClient(public.ClientConfig{URL: apiURL}, public.WithDialFunc(dialFn))
		require.NoError(t, err)
		require.NotNil(t, c)
		require.NotEmpty(t, c)
		require.Equal(t, apiURL, c.URL())

		_ = c.Register("", "")

		require.True(t, called)
	})
}

func TestClientRegister(t *testing.T) {
	th := SetupTestHelper(t, nil)
	defer th.Teardown()

	c, err := public.NewClient(public.ClientConfig{
		URL:     th.apiURL,
		AuthKey: th.srvc.cfg.API.Security.AdminSecretKey,
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	defer c.Close()

	t.Run("empty clientID", func(t *testing.T) {
		authKey, err := random.NewSecureString(auth.MinKeyLen)
		require.NoError(t, err)
		err = c.Register("", authKey)
		require.Error(t, err)
		require.Equal(t, "request failed: registration failed: error: empty key", err.Error())
	})

	t.Run("empty authKey", func(t *testing.T) {
		err := c.Register("clientA", "")
		require.Error(t, err)
		require.EqualError(t, err, "request failed: registration failed: key not long enough")
	})

	t.Run("valid", func(t *testing.T) {
		authKey, err := random.NewSecureString(auth.MinKeyLen)
		require.NoError(t, err)
		err = c.Register("clientA", authKey)
		require.NoError(t, err)
	})

	t.Run("existing clientID", func(t *testing.T) {
		authKey, err := random.NewSecureString(auth.MinKeyLen)
		require.NoError(t, err)
		err = c.Register("clientA", authKey)
		require.Error(t, err)
		require.Equal(t, "request failed: registration failed: already registered", err.Error())
	})

	t.Run("unauthorized", func(t *testing.T) {
		c, err := public.NewClient(public.ClientConfig{
			URL:     th.apiURL,
			AuthKey: th.srvc.cfg.API.Security.AdminSecretKey + "_",
		})
		require.NoError(t, err)
		require.NotNil(t, c)
		defer c.Close()

		err = c.Register("", "")
		require.Error(t, err)
		require.Equal(t, public.ErrUnauthorized, err)
	})

	t.Run("self registering", func(t *testing.T) {
		c, err := public.NewClient(public.ClientConfig{
			URL: th.apiURL,
		})
		require.NoError(t, err)
		require.NotNil(t, c)
		defer c.Close()

		authKey, err := random.NewSecureString(auth.MinKeyLen)
		require.NoError(t, err)
		err = c.Register("clientB", authKey)
		require.Error(t, err)
		require.Equal(t, public.ErrUnauthorized, err)

		th.srvc.cfg.API.Security.AllowSelfRegistration = true
		err = c.Register("clientB", authKey)
		require.NoError(t, err)
	})
}

func TestClientUnregister(t *testing.T) {
	th := SetupTestHelper(t, nil)
	defer th.Teardown()

	c, err := public.NewClient(public.ClientConfig{
		URL:     th.apiURL,
		AuthKey: th.srvc.cfg.API.Security.AdminSecretKey,
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	defer c.Close()

	t.Run("empty client ID", func(t *testing.T) {
		authKey, err := random.NewSecureString(auth.MinKeyLen)
		require.NoError(t, err)
		err = c.Register("clientA", authKey)
		require.NoError(t, err)
		require.NotEmpty(t, authKey)

		err = c.Unregister("")
		require.Error(t, err)
		require.Equal(t, "request failed: client id should not be empty", err.Error())
	})

	t.Run("not found", func(t *testing.T) {
		err := c.Unregister("clientB")
		require.Error(t, err)
		require.Equal(t, "request failed: unregister failed: error: not found", err.Error())
	})

	t.Run("success", func(t *testing.T) {
		err := c.Unregister("clientA")
		require.NoError(t, err)
	})

	t.Run("unauthorized", func(t *testing.T) {
		c, err := public.NewClient(public.ClientConfig{
			URL:     th.apiURL,
			AuthKey: th.srvc.cfg.API.Security.AdminSecretKey + "_",
		})
		require.NoError(t, err)
		require.NotNil(t, c)
		defer c.Close()

		err = c.Unregister("clientA")
		require.Error(t, err)
		require.Equal(t, public.ErrUnauthorized, err)
	})
}

func TestClientLogin(t *testing.T) {
	th := SetupTestHelper(t, nil)
	defer th.Teardown()

	c, err := public.NewClient(public.ClientConfig{
		URL:     th.apiURL,
		AuthKey: th.srvc.cfg.API.Security.AdminSecretKey,
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	defer c.Close()

	t.Run("success", func(t *testing.T) {
		authKey, err := random.NewSecureString(auth.MinKeyLen)
		require.NoError(t, err)
		err = c.Register("clientA", authKey)
		require.NoError(t, err)
		require.NotEmpty(t, authKey)

		err = c.Login("clientA", authKey)
		require.NoError(t, err)
		require.NotEmpty(t, c.AuthToken())
	})

	t.Run("not found", func(t *testing.T) {
		err := c.Login("clientC", "authKey")
		require.Error(t, err)
		require.Equal(t, "request failed: login failed: authentication failed: error: not found", err.Error())
	})

	t.Run("auth failed", func(t *testing.T) {
		authKey, err := random.NewSecureString(auth.MinKeyLen)
		require.NoError(t, err)
		err = c.Register("clientB", authKey)
		require.NoError(t, err)
		require.NotEmpty(t, authKey)

		err = c.Login("clientB", authKey+"bad")
		require.Error(t, err)
		require.Equal(t, "request failed: login failed: authentication failed", err.Error())
	})
}

func TestClientGetVersionInfo(t *testing.T) {
	th := SetupTestHelper(t, nil)
	defer th.Teardown()

	c, err := public.NewClient(public.ClientConfig{
		URL:     th.apiURL,
		AuthKey: th.srvc.cfg.API.Security.AdminSecretKey,
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	defer c.Close()

	info, err := c.GetVersionInfo()
	require.NoError(t, err)
	require.NotEmpty(t, info)
	require.Equal(t, getVersionInfo(), info)
}
