package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This command is an explicit recipe for the reviewed fixture set, not an importer.
// Updating the manifest requires a scoped rights/identity review and recipe update.
const manifestSHA256 = "512f67ffecd6fadba712c2b6a09db5e237a152e401e9792eaee89269f6c44561"

type fileRecord struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type fixture struct {
	fileRecord
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Creator        string    `json:"creator"`
	SourceProvider string    `json:"source_provider"`
	SourceURL      string    `json:"source_url"`
	EvidenceURL    string    `json:"license_evidence_url"`
	EvidencePath   string    `json:"evidence_path"`
	DownloadURL    string    `json:"download_url"`
	LicenseID      string    `json:"license_id"`
	AcquiredAt     time.Time `json:"acquired_at_utc"`
	Attribution    string    `json:"attribution"`
	Modifications  string    `json:"modifications"`
	MIME           string    `json:"media_type"`
	Codec          string    `json:"codec"`
	Seconds        float64   `json:"frame_duration_seconds"`
	Status         string    `json:"status"`
}
type license struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	URL         string   `json:"url"`
	Territory   string   `json:"territory"`
	Term        string   `json:"term"`
	Conditions  []string `json:"conditions"`
	Limitations string   `json:"limitations"`
}
type manifest struct {
	VerifiedOn string       `json:"verified_on"`
	VerifiedBy string       `json:"verified_by"`
	Decision   string       `json:"fixture_rights_decision"`
	Gate       string       `json:"production_release_gate"`
	Licenses   []license    `json:"licenses"`
	Assets     []fixture    `json:"assets"`
	Retained   []fileRecord `json:"retained_files"`
}
type seedAsset struct {
	Artist, Track, Asset map[string]any
	Data                 []byte
	Key                  string
}

// Bound reads and reject symlinks/special files in addition to os.Root confinement.
func readRegular(root *os.Root, name string, limit int64) ([]byte, error) {
	if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("unsafe fixture path %q", name)
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return nil, fmt.Errorf("missing file %s", name)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink not allowed: %s", name)
		}
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s", name)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid file type/size: %s", name)
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, fmt.Errorf("cannot read bounded file %s", name)
	}
	return data, nil
}

func verifyFile(root *os.Root, r fileRecord) ([]byte, error) {
	data, err := readRegular(root, r.Path, 2_000_000)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != r.Bytes || fmt.Sprintf("%x", sha256.Sum256(data)) != r.SHA256 {
		return nil, fmt.Errorf("size/SHA-256 mismatch: %s", r.Path)
	}
	return data, nil
}

func stableID(kind, key string) string {
	sum := sha256.Sum256([]byte("sonora-development-fixtures-v1/" + kind + "/" + key))
	sum[6] = (sum[6] & 15) | 128 // UUID version 8, deterministic recipe identity.
	sum[8] = (sum[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func loadFixtures(dir string) ([]seedAsset, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot open fixture directory")
	}
	defer root.Close()
	raw, err := readRegular(root, "manifest.json", 100_000)
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != manifestSHA256 {
		return nil, fmt.Errorf("fixture manifest differs from reviewed recipe; stop and review, do not automatically update the pin")
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid fixture manifest")
	}
	if len(m.Assets) != 3 || m.Decision != "verified_for_stated_uses" {
		return nil, fmt.Errorf("fixture set is not approved")
	}
	reviewed, err := time.Parse("2006-01-02", m.VerifiedOn)
	if err != nil {
		return nil, fmt.Errorf("invalid fixture review date")
	}
	for _, r := range m.Retained {
		if _, err := verifyFile(root, r); err != nil {
			return nil, err
		}
	}
	result := make([]seedAsset, 0, len(m.Assets))
	for _, a := range m.Assets {
		if a.Status != "included" {
			return nil, fmt.Errorf("fixture %s is not included", a.ID)
		}
		data, err := verifyFile(root, a.fileRecord)
		if err != nil {
			return nil, err
		}
		var l license
		for _, candidate := range m.Licenses {
			if candidate.ID == a.LicenseID {
				l = candidate
			}
		}
		if l.ID == "" {
			return nil, fmt.Errorf("missing license for %s", a.ID)
		}
		artistID, trackID, assetID := stableID("artist", a.Creator), stableID("track", a.ID), stableID("asset", a.ID)
		key := assetID + ".mp3"
		duration := int64(math.Round(a.Seconds * 1000))
		result = append(result, seedAsset{
			Artist: map[string]any{"id": artistID, "name": a.Creator},
			Track: map[string]any{"id": trackID, "primary_artist_id": artistID, "album_id": nil, "title": a.Title,
				"display_credits": a.Attribution, "duration_ms": duration, "disc_number": nil, "track_number": nil},
			Asset: map[string]any{"id": assetID, "track_id": trackID, "storage_key": key, "sha256": a.SHA256,
				"mime_type": a.MIME, "codec": a.Codec, "byte_length": a.Bytes, "duration_ms": duration, "acquired_at": a.AcquiredAt,
				"source_provider": a.SourceProvider, "source_url": a.SourceURL, "download_reference": a.DownloadURL,
				"creator_credits": a.Creator, "rights_holder_credits": a.Creator + " (credited creator/publisher in retained evidence; no independent ownership warranty)",
				"license_name": l.Name, "license_version": l.Version, "license_url": l.URL, "evidence_url": a.EvidenceURL,
				"evidence_reference": "fixtures/audio/" + a.EvidencePath, "reviewed_by": m.VerifiedBy, "reviewed_at": reviewed,
				"review_rationale": "Existing Task 1 asset-specific fixture decision; docs/LICENSED_AUDIO.md; " + m.Decision + "; review date has day precision.",
				// These values describe ONLY the three pinned asset-specific fixture decisions.
				"hosting_permitted": true, "playback_permitted": true, "redistribution_permitted": true, "commercial_use_permitted": true,
				"audience_scope":  "Development repository/fixture archives and local testing only; " + m.Gate,
				"territory_scope": l.Territory, "license_term": l.Term, "license_expires_at": nil,
				"use_conditions": strings.Join(l.Conditions, "\n") + "\n" + l.Limitations +
					"\nKeep fixtures/audio/README.md, manifest, evidence and license texts with distributions. Frame-derived duration estimate; full decode/browser acceptance pending.",
				"attribution": a.Attribution, "modifications": a.Modifications},
			Data: data, Key: key,
		})
	}
	return result, nil
}
