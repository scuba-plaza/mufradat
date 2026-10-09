package googleauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/scuba-plaza/mufradat/internal/fsutil"
)

const Scope = "https://www.googleapis.com/auth/cloud-platform"

var ErrNone = errors.New("no Google credentials found")

type Candidate struct {
	Source   string
	Path     string
	Required bool
}

type Credentials struct {
	JSON   []byte
	Source string
	Path   string
}

type Info struct {
	Type      string
	Account   string
	ProjectID string
}

func Resolve(candidates []Candidate) (Credentials, error) {
	for _, c := range candidates {
		if c.Path == "" {
			continue
		}
		raw, err := fsutil.ReadFileIfExists(c.Path)
		if err != nil {
			return Credentials{}, fmt.Errorf("%s: %w", c.Source, err)
		}
		if raw == nil {
			if c.Required {
				return Credentials{}, fmt.Errorf("%s points at %s, which does not exist", c.Source, c.Path)
			}
			continue
		}
		return Credentials{JSON: raw, Source: c.Source, Path: c.Path}, nil
	}
	return Credentials{}, ErrNone
}

type keyFile struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
	ProjectID   string `json:"project_id"`
	QuotaProj   string `json:"quota_project_id"`
}

func parse(ctx context.Context, raw []byte, kind string) (*google.Credentials, error) {
	switch t := google.CredentialsType(kind); t {
	case google.ServiceAccount, google.AuthorizedUser:
		return google.CredentialsFromJSONWithTypeAndParams(ctx, raw, t, google.CredentialsParams{Scopes: []string{Scope}})
	}
	return nil, fmt.Errorf("credentials of type %q are not supported; use a service-account key or gcloud application-default credentials", kind)
}

func Validate(ctx context.Context, raw []byte) (Info, error) {
	var k keyFile
	if err := json.Unmarshal(raw, &k); err != nil {
		return Info{}, fmt.Errorf("not a JSON credentials file: %w", err)
	}
	if k.Type == "" {
		return Info{}, errors.New("the file has no \"type\" field, so it is not a Google credentials file")
	}
	if _, err := parse(ctx, raw, k.Type); err != nil {
		return Info{}, err
	}
	info := Info{Type: k.Type, Account: k.ClientEmail, ProjectID: k.ProjectID}
	if info.ProjectID == "" {
		info.ProjectID = k.QuotaProj
	}
	return info, nil
}

func Import(ctx context.Context, src, dest string) (Info, error) {
	raw, err := fsutil.ReadFileIfExists(src)
	if err != nil {
		return Info{}, err
	}
	if raw == nil {
		return Info{}, fmt.Errorf("%s does not exist", src)
	}
	info, err := Validate(ctx, raw)
	if err != nil {
		return Info{}, fmt.Errorf("%s: %w", src, err)
	}
	if err := fsutil.WriteFileAtomic(dest, raw, 0o600); err != nil {
		return Info{}, err
	}
	return info, nil
}

type quotaTransport struct {
	base    http.RoundTripper
	project string
}

func (t quotaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("x-goog-user-project", t.project)
	return t.base.RoundTrip(r)
}

func Transport(ctx context.Context, base http.RoundTripper, c Credentials, quotaProject string) (http.RoundTripper, error) {
	if base == nil {
		base = http.DefaultTransport
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: base})
	var k keyFile
	if err := json.Unmarshal(c.JSON, &k); err != nil {
		return nil, fmt.Errorf("%s: not a JSON credentials file: %w", c.Source, err)
	}
	creds, err := parse(ctx, c.JSON, k.Type)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.Source, err)
	}
	var rt http.RoundTripper = &oauth2.Transport{Source: creds.TokenSource, Base: base}
	if quotaProject != "" {
		rt = quotaTransport{base: rt, project: quotaProject}
	}
	return rt, nil
}
