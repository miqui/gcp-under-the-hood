// Package supply holds pure logic for image digests and attestation payloads.
package supply

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var digestRe = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

const payloadType = "Google cloud binauthz container signature"

// ImageRef is a parsed image reference pinned by digest.
type ImageRef struct {
	Registry string
	Path     string
	Digest   string
}

// ParseDigestRef parses "host/path@sha256:<64 hex>". Tags are rejected
// because Binary Authorization attestations bind to digests, not tags.
func ParseDigestRef(ref string) (ImageRef, error) {
	at := strings.Index(ref, "@")
	if at < 0 {
		return ImageRef{}, fmt.Errorf("reference %q is not pinned by digest (tags are mutable)", ref)
	}
	name, digest := ref[:at], ref[at+1:]
	if !digestRe.MatchString(digest) {
		return ImageRef{}, fmt.Errorf("invalid digest %q: want sha256:<64 lowercase hex>", digest)
	}
	slash := strings.Index(name, "/")
	if slash <= 0 || slash == len(name)-1 {
		return ImageRef{}, fmt.Errorf("reference %q needs registry host and path", ref)
	}
	return ImageRef{Registry: name[:slash], Path: name[slash+1:], Digest: digest}, nil
}

// String returns the canonical reference.
func (r ImageRef) String() string { return r.Registry + "/" + r.Path + "@" + r.Digest }

// Payload is the Binary Authorization "simple signing" critical payload.
type Payload struct {
	Critical Critical `json:"critical"`
}

// Critical is the signed part of the payload.
type Critical struct {
	Identity Identity `json:"identity"`
	Image    Image    `json:"image"`
	Type     string   `json:"type"`
}

// Identity names the image repository.
type Identity struct {
	DockerReference string `json:"docker-reference"`
}

// Image carries the manifest digest.
type Image struct {
	DockerManifestDigest string `json:"docker-manifest-digest"`
}

// BuildPayload returns the JSON that an attestor signs for the image.
func BuildPayload(r ImageRef) ([]byte, error) {
	p := Payload{Critical: Critical{
		Identity: Identity{DockerReference: r.Registry + "/" + r.Path},
		Image:    Image{DockerManifestDigest: r.Digest},
		Type:     payloadType,
	}}
	return json.Marshal(p)
}

// ValidatePayload checks that payload JSON matches the expected image.
func ValidatePayload(data []byte, want ImageRef) error {
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("payload is not valid JSON: %w", err)
	}
	if p.Critical.Type != payloadType {
		return fmt.Errorf("unexpected type %q", p.Critical.Type)
	}
	if p.Critical.Image.DockerManifestDigest != want.Digest {
		return fmt.Errorf("digest mismatch: payload %q, image %q", p.Critical.Image.DockerManifestDigest, want.Digest)
	}
	if p.Critical.Identity.DockerReference != want.Registry+"/"+want.Path {
		return fmt.Errorf("docker-reference mismatch: %q", p.Critical.Identity.DockerReference)
	}
	return nil
}

// Split is one traffic target of a Cloud Run service.
type Split struct {
	Revision string
	Percent  int
}

// ValidateSplits ensures percents are 0..100 and sum to exactly 100.
func ValidateSplits(s []Split) error {
	total := 0
	for _, x := range s {
		if x.Percent < 0 || x.Percent > 100 {
			return fmt.Errorf("revision %q percent %d out of range", x.Revision, x.Percent)
		}
		total += x.Percent
	}
	if total != 100 {
		return fmt.Errorf("traffic sums to %d, want 100", total)
	}
	return nil
}
