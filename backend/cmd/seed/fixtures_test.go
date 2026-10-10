package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixtureDir() string { return filepath.Join("..", "..", "..", "fixtures", "audio") }

func copyFixtures(t *testing.T, dest string) {
	t.Helper()
	err := filepath.WalkDir(fixtureDir(), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixtureDir(), p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestFixtureMetadata(t *testing.T) {
	assets, err := loadFixtures(fixtureDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 3 {
		t.Fatal("expected three fixtures")
	}
	artists := map[any]bool{}
	for _, a := range assets {
		artists[a.Artist["id"]] = true
		if a.Track["album_id"] != nil || a.Track["disc_number"] != nil || a.Track["track_number"] != nil {
			t.Fatal("invented album/position")
		}
		if a.Asset["track_id"] != a.Track["id"] || a.Track["primary_artist_id"] != a.Artist["id"] ||
			a.Track["duration_ms"] != a.Asset["duration_ms"] || a.Key != a.Asset["id"].(string)+".mp3" {
			t.Fatal("inconsistent identities/metadata")
		}
	}
	if len(artists) != 2 {
		t.Fatal("expected two credited artists")
	}
	if assets[0].Track["title"] != "Pong" || assets[0].Track["duration_ms"] != int64(14040) ||
		assets[1].Asset["license_version"] != "4.0" || assets[2].Track["duration_ms"] != int64(16431) {
		t.Fatal("fixture metadata changed")
	}
	again, err := loadFixtures(fixtureDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(assets)
	b, _ := json.Marshal(again)
	if !bytes.Equal(a, b) {
		t.Fatal("recipe is not deterministic")
	}
}
func TestFixtureFailures(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "truncated", "evidence", "metadata", "withdrawn"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			copyFixtures(t, dir)
			p := filepath.Join(dir, "disco-sting.mp3")
			switch kind {
			case "missing":
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				data, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				data[len(data)-1] ^= 1
				if err := os.WriteFile(p, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "truncated":
				if err := os.WriteFile(p, []byte("bad"), 0600); err != nil {
					t.Fatal(err)
				}
			case "evidence":
				if err := os.Remove(filepath.Join(dir, "evidence", "pong-0.json")); err != nil {
					t.Fatal(err)
				}
			default:
				p = filepath.Join(dir, "manifest.json")
				data, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "metadata" {
					data = bytes.Replace(data, []byte("Pong"), []byte("Wrong"), 1)
				} else {
					data = bytes.ReplaceAll(data, []byte(`"included"`), []byte(`"withdrawn"`))
				}
				if err := os.WriteFile(p, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := loadFixtures(dir); err == nil {
				t.Fatal("invalid fixture accepted")
			}
		})
	}
}
func TestConfinedPaths(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, name := range []string{"../outside.mp3", "/outside.mp3", "..\\outside.mp3"} {
		if _, err := readRegular(root, name, 100); err == nil {
			t.Fatalf("unsafe path accepted: %s", name)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "real"), []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "linked")); err != nil {
		t.Skip("symlink creation unavailable: ", err)
	}
	if _, err := readRegular(root, "linked", 100); err == nil {
		t.Fatal("symlink accepted")
	}
}
func TestCommandArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"production"}, {"development", "extra"}} {
		if err := run(context.Background(), args, &bytes.Buffer{}); err == nil {
			t.Fatal("expected usage error")
		}
	}
}
