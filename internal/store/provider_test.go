package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
)

func TestProviderOrderBizflyFirst(t *testing.T) {
	assert.Equal(t, []string{"bizfly", "aws", "s3"}, store.Providers.IDs())
	assert.Equal(t, "Bizfly Cloud", store.Providers.List()[0].Title)
}

func TestProviderFromEndpoint(t *testing.T) {
	assert.Equal(t, store.ProviderAWS, store.Providers.FromEndpoint("").ID)
	assert.Equal(t, store.ProviderBizfly, store.Providers.FromEndpoint("https://hcm.ss.bfcplatform.vn").ID)
	assert.Equal(t, store.ProviderBizfly, store.Providers.FromEndpoint("https://HCM.ss.bfcplatform.vn/").ID)
	assert.Equal(t, store.ProviderOther, store.Providers.FromEndpoint("http://minio.local:9000").ID)
}

func TestProviderParseURL(t *testing.T) {
	bizfly := "https://hcm.ss.bfcplatform.vn"
	cases := []struct {
		raw              string
		provider         store.ProviderID
		endpoint, bucket string
	}{
		{"https://hop-cli.hcm.ss.bfcplatform.vn", store.ProviderBizfly, bizfly, "hop-cli"},
		{"https://hop-cli.hcm.ss.bfcplatform.vn/", store.ProviderBizfly, bizfly, "hop-cli"},
		{" https://hcm.ss.bfcplatform.vn/hop-cli ", store.ProviderBizfly, bizfly, "hop-cli"},
		{"http://minio.local:9000/backups", store.ProviderOther, "http://minio.local:9000", "backups"},
	}
	for _, c := range cases {
		loc, err := store.Providers.ParseURL(c.raw)
		require.NoError(t, err, c.raw)
		assert.Equal(t, c.provider, loc.Provider.ID, c.raw)
		assert.Equal(t, c.endpoint, loc.Endpoint, c.raw)
		assert.Equal(t, c.bucket, loc.Bucket, c.raw)
	}
	for _, bad := range []string{"hop-cli", "https://hcm.ss.bfcplatform.vn", "https://minio.local", "https://minio.local/a/b", "::"} {
		_, err := store.Providers.ParseURL(bad)
		assert.Equal(t, apperr.ExitUsage, apperr.CodeOf(err), bad)
	}
}

func TestProviderLocateAcceptsNameOrURL(t *testing.T) {
	bizfly := store.Providers.List()[0]
	loc, err := store.Providers.Locate(bizfly, bizfly.Endpoint, "hop-cli")
	require.NoError(t, err)
	assert.Equal(t, store.Location{Provider: bizfly, Endpoint: bizfly.Endpoint, Bucket: "hop-cli"}, loc)
	loc, err = store.Providers.Locate(bizfly, bizfly.Endpoint, "https://other.hcm.ss.bfcplatform.vn")
	require.NoError(t, err)
	assert.Equal(t, "other", loc.Bucket)
}

func TestProviderLookup(t *testing.T) {
	p, err := store.Providers.Lookup("bizfly")
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", p.Region)
	_, err = store.Providers.Lookup("gcs")
	assert.EqualError(t, err, `unknown provider "gcs" — use one of: bizfly, aws, s3`)
}
