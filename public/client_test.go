// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package public

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTLSServerCert(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	certFile, err := os.CreateTemp("", "test_ca_*.pem")
	require.NoError(t, err)
	cert := ts.Certificate()
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	require.NoError(t, err)
	require.NoError(t, certFile.Close())
	t.Cleanup(func() { os.Remove(certFile.Name()) })
	return certFile.Name()
}

func TestNewClientTLS(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	validCertFile := writeTLSServerCert(t, ts)

	t.Run("ca cert file - connects successfully", func(t *testing.T) {
		t.Setenv("API_HTTP_TLS_CA_CERT_FILE", validCertFile)

		c, err := NewClient(ClientConfig{URL: ts.URL})
		require.NoError(t, err)
		require.NotNil(t, c)

		resp, err := c.httpClient.Get(ts.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ca cert file - missing file", func(t *testing.T) {
		t.Setenv("API_HTTP_TLS_CA_CERT_FILE", "/nonexistent/cert.pem")

		c, err := NewClient(ClientConfig{URL: ts.URL})
		require.EqualError(t, err, "failed to read CA certificate: open /nonexistent/cert.pem: no such file or directory")
		require.Nil(t, c)
	})

	t.Run("ca cert file - invalid PEM", func(t *testing.T) {
		f, err := os.CreateTemp("", "bad_cert_*.pem")
		require.NoError(t, err)
		_, err = f.WriteString("not a valid certificate")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		t.Cleanup(func() { os.Remove(f.Name()) })

		t.Setenv("API_HTTP_TLS_CA_CERT_FILE", f.Name())

		c, err := NewClient(ClientConfig{URL: ts.URL})
		require.EqualError(t, err, "failed to parse CA certificate")
		require.Nil(t, c)
	})

	t.Run("insecure skip verify - connects successfully", func(t *testing.T) {
		t.Setenv("API_HTTP_TLS_INSECURE_SKIP_VERIFY", "true")

		c, err := NewClient(ClientConfig{URL: ts.URL})
		require.NoError(t, err)
		require.NotNil(t, c)

		resp, err := c.httpClient.Get(ts.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("insecure skip verify - value 1", func(t *testing.T) {
		t.Setenv("API_HTTP_TLS_INSECURE_SKIP_VERIFY", "1")

		c, err := NewClient(ClientConfig{URL: ts.URL})
		require.NoError(t, err)
		require.NotNil(t, c)

		resp, err := c.httpClient.Get(ts.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("no tls config - fails without cert", func(t *testing.T) {
		c, err := NewClient(ClientConfig{URL: ts.URL})
		require.NoError(t, err)
		require.NotNil(t, c)

		// Default transport should reject self-signed cert
		_, err = c.httpClient.Get(ts.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "certificate")
	})
}
