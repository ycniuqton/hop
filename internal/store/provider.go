package store

import (
	"net/url"
	"strings"

	"github.com/ycniuqton/hop/internal/apperr"
)

// ProviderID names a storage provider in flags and JSON output.
type ProviderID string

const (
	ProviderBizfly ProviderID = "bizfly"
	ProviderAWS    ProviderID = "aws"
	ProviderOther  ProviderID = "s3"
)

// KeyLabels are the prompt labels a provider uses for its access keys.
type KeyLabels struct {
	Access, Secret string
}

// Provider is an S3-compatible service; its fields drive what `hop init` still has to ask.
// An empty Endpoint means the SDK default (AsksEndpoint false) or a prompt (AsksEndpoint true);
// an empty Region means a prompt.
type Provider struct {
	ID           ProviderID
	Title        string
	Endpoint     string
	Region       string
	AsksEndpoint bool
	BucketPrompt string
	Keys         KeyLabels
}

// Location is where a workspace lives: provider, endpoint and bucket.
type Location struct {
	Provider Provider
	Endpoint string
	Bucket   string
}

// URLParts are the pieces of a bucket URL hop understands.
var URLParts = struct{ Separator, PathSep, HostSep, DefaultScheme string }{
	Separator: "://", PathSep: "/", HostSep: ".", DefaultScheme: "https",
}

// ProviderManager is the provider registry; adding a provider means adding one entry.
type ProviderManager struct {
	list     []Provider
	fallback ProviderID
	other    ProviderID
}

var genericKeys = KeyLabels{Access: "Access key", Secret: "Secret key"}

// Providers is the built-in registry, in prompt order: Bizfly Cloud first and default.
var Providers = ProviderManager{fallback: ProviderAWS, other: ProviderOther, list: []Provider{
	{ID: ProviderBizfly, Title: "Bizfly Cloud", Endpoint: "https://hcm.ss.bfcplatform.vn", Region: "us-east-1",
		BucketPrompt: "Bucket URL or name", Keys: genericKeys},
	{ID: ProviderAWS, Title: "Amazon S3", BucketPrompt: "S3 bucket",
		Keys: KeyLabels{Access: "AWS access key ID", Secret: "AWS secret access key"}},
	{ID: ProviderOther, Title: "Other S3-compatible", Region: "us-east-1", AsksEndpoint: true,
		BucketPrompt: "Bucket name", Keys: genericKeys},
}}

// List returns providers in prompt order.
func (m ProviderManager) List() []Provider { return m.list }

// IDs lists provider IDs for flag help and errors.
func (m ProviderManager) IDs() []string {
	out := make([]string, len(m.list))
	for i, p := range m.list {
		out[i] = string(p.ID)
	}
	return out
}

// Lookup finds a provider by ID.
func (m ProviderManager) Lookup(id string) (Provider, error) {
	for _, p := range m.list {
		if string(p.ID) == id {
			return p, nil
		}
	}
	return Provider{}, apperr.New(apperr.ExitUsage, "unknown provider %q", id).
		WithHint("use one of: %s", strings.Join(m.IDs(), ", "))
}

func (m ProviderManager) byID(id ProviderID) Provider {
	p, _ := m.Lookup(string(id))
	return p
}

// FromEndpoint names the provider behind a configured endpoint ("" is Amazon S3).
func (m ProviderManager) FromEndpoint(endpoint string) Provider {
	if endpoint == "" {
		return m.byID(m.fallback)
	}
	for _, p := range m.list {
		if p.Endpoint != "" && strings.EqualFold(strings.TrimSuffix(endpoint, URLParts.PathSep), p.Endpoint) {
			return p
		}
	}
	return m.byID(m.other)
}

// ParseURL reads a bucket URL, virtual-hosted (https://b.host) or path-style (https://host/b).
func (m ProviderManager) ParseURL(raw string) (Location, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || !strings.Contains(raw, URLParts.Separator) {
		return Location{}, m.badURL(raw)
	}
	for _, p := range m.list {
		if loc, ok := m.match(p, u); ok {
			return loc, nil
		}
	}
	bucket := strings.Trim(u.Path, URLParts.PathSep)
	if bucket == "" || strings.Contains(bucket, URLParts.PathSep) {
		return Location{}, m.badURL(raw)
	}
	return Location{Provider: m.byID(m.other), Endpoint: u.Scheme + URLParts.Separator + u.Host, Bucket: bucket}, nil
}

func (m ProviderManager) match(p Provider, u *url.URL) (Location, bool) {
	ep, err := url.Parse(p.Endpoint)
	if p.Endpoint == "" || err != nil {
		return Location{}, false
	}
	host, base := strings.ToLower(u.Host), strings.ToLower(ep.Host)
	bucket := strings.Trim(u.Path, URLParts.PathSep)
	switch {
	case strings.HasSuffix(host, URLParts.HostSep+base) && bucket == "":
		bucket = strings.TrimSuffix(host, URLParts.HostSep+base)
	case host != base || strings.Contains(bucket, URLParts.PathSep):
		return Location{}, false
	}
	return Location{Provider: p, Endpoint: p.Endpoint, Bucket: bucket}, bucket != ""
}

func (m ProviderManager) badURL(raw string) error {
	return apperr.New(apperr.ExitUsage, "cannot read a bucket from %q", raw).
		WithHint("use a URL like https://my-bucket.hcm.ss.bfcplatform.vn, or pass --endpoint and --bucket")
}

// Endpoint normalizes a typed endpoint: https:// is assumed and a trailing slash dropped.
func (m ProviderManager) Endpoint(raw string) string {
	ep := strings.TrimSuffix(strings.TrimSpace(raw), URLParts.PathSep)
	if ep != "" && !strings.Contains(ep, URLParts.Separator) {
		ep = URLParts.DefaultScheme + URLParts.Separator + ep
	}
	return ep
}

// Locate turns a bucket answer into a location: a bucket URL is parsed, a plain name keeps p.
func (m ProviderManager) Locate(p Provider, endpoint, answer string) (Location, error) {
	if strings.Contains(answer, URLParts.Separator) {
		return m.ParseURL(answer)
	}
	return Location{Provider: p, Endpoint: endpoint, Bucket: strings.TrimSpace(answer)}, nil
}
