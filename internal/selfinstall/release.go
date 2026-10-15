// Package selfinstall lets hop update and uninstall itself: GitHub releases, version order,
// atomic self-replacement and the list of files hop leaves on a machine.
package selfinstall

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Defaults describe the official release channel and its asset names (they match .goreleaser.yaml).
var Defaults = struct {
	Base, Repo, Binary, Checksums, TagPath, LatestPath, DownloadPath string
	Timeout                                                          time.Duration
	MaxDownload                                                      int64
}{
	Base: "https://github.com", Repo: "ycniuqton/hop", Binary: "hop", Checksums: "checksums.txt",
	TagPath: "/releases/tag/", LatestPath: "/releases/latest", DownloadPath: "/releases/download/",
	Timeout: 2 * time.Minute, MaxDownload: 200 << 20,
}

// Releases reads published versions and their binaries from a GitHub-style release host.
type Releases struct {
	Base   string
	Repo   string
	Client *http.Client
}

// NewReleases fills in the official channel for empty values.
func NewReleases(base, repo string) Releases {
	if base == "" {
		base = Defaults.Base
	}
	if repo == "" {
		repo = Defaults.Repo
	}
	return Releases{Base: strings.TrimSuffix(base, "/"), Repo: repo, Client: &http.Client{Timeout: Defaults.Timeout}}
}

// Latest returns the newest tag by reading the redirect of /releases/latest (no API rate limit).
func (r Releases) Latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.Base+"/"+r.Repo+Defaults.LatestPath, nil)
	if err != nil {
		return "", err
	}
	client := *r.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return "", r.networkError(err)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, Defaults.TagPath) {
		return "", apperr.New(apperr.ExitNotFound, "no release found at %s/%s (HTTP %d)", r.Base, r.Repo, resp.StatusCode)
	}
	return path.Base(loc), nil
}

// Asset is the archive name for one platform, e.g. hop_linux_amd64.tar.gz or hop_windows_amd64.zip.
func (r Releases) Asset(goos, goarch string) string {
	return Defaults.Binary + "_" + goos + "_" + goarch + PackagingFor(goos).Format.Ext()
}

// Fetch downloads the platform archive for tag, checks it against checksums.txt and returns the binary.
func (r Releases) Fetch(ctx context.Context, tag, goos, goarch string) ([]byte, error) {
	asset := r.Asset(goos, goarch)
	archive, err := r.download(ctx, tag, asset)
	if err != nil {
		return nil, err
	}
	sums, err := r.download(ctx, tag, Defaults.Checksums)
	if err != nil {
		return nil, err
	}
	if err := r.verify(sums, asset, archive); err != nil {
		return nil, err
	}
	pk := PackagingFor(goos)
	return pk.Format.Extract(archive, pk.Executable)
}

func (r Releases) download(ctx context.Context, tag, name string) ([]byte, error) {
	url := r.Base + "/" + r.Repo + Defaults.DownloadPath + tag + "/" + name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, r.networkError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, apperr.New(apperr.ExitNotFound, "%s has no %s", tag, name).
			WithHint("this platform may not be released; see %s/%s/releases", r.Base, r.Repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, apperr.New(apperr.ExitNetwork, "download %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, Defaults.MaxDownload))
	if err != nil {
		return nil, r.networkError(err)
	}
	return data, nil
}

func (r Releases) verify(sums []byte, asset string, archive []byte) error {
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == asset && fields[0] == got {
			return nil
		}
	}
	return apperr.New(apperr.ExitGeneral, "checksum mismatch for %s — refusing to install it", asset)
}

func (r Releases) networkError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return apperr.New(apperr.ExitNetwork, "cannot reach %s: %v", r.Base, err).Wrap(err)
}

// Version is a release version x.y.z with an optional pre-release suffix (e.g. 0.3.0-dev).
type Version struct {
	Major, Minor, Patch int
	Pre                 string
	Valid               bool
}

// ParseVersion reads "v1.2.3", "1.2.3" or "1.2.3-dev"; anything else (e.g. "dev") is not Valid.
func ParseVersion(s string) Version {
	core, pre, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(s), "v"), "-")
	var v Version
	n, err := fmt.Sscanf(core, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	v.Pre, v.Valid = pre, err == nil && n == 3 && fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) == core
	return v
}

// Compare orders versions; an invalid version (a plain dev build) sorts before every release.
func (v Version) Compare(o Version) int {
	if v.Valid != o.Valid {
		return map[bool]int{true: 1, false: -1}[v.Valid]
	}
	for _, d := range []int{v.Major - o.Major, v.Minor - o.Minor, v.Patch - o.Patch} {
		if d != 0 {
			return map[bool]int{true: 1, false: -1}[d > 0]
		}
	}
	switch {
	case v.Pre == o.Pre:
		return 0
	case v.Pre == "":
		return 1
	case o.Pre == "":
		return -1
	}
	return strings.Compare(v.Pre, o.Pre)
}
