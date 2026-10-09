package main

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/scuba-plaza/mufradat/internal/fsutil"
	"github.com/scuba-plaza/mufradat/internal/jsonl"
)

type source struct {
	Name     string
	URL      string
	File     string
	ZipEntry string
	MaxLines int
}

var sources = []source{
	{Name: "Wiktionary Arabic (kaikki.org)", URL: "https://kaikki.org/dictionary/Arabic/kaikki.org-dictionary-Arabic.jsonl", File: "kaikki-ar.jsonl"},
	{Name: "OpenSubtitles frequency list", URL: "https://raw.githubusercontent.com/hermitdave/FrequencyWords/master/content/2018/ar/ar_50k.txt", File: "subtitles-ar-50k.txt"},
	{Name: "CAMeL MSA frequency list", URL: "https://github.com/CAMeL-Lab/Camel_Arabic_Frequency_Lists/releases/download/v1.0/MSA_freq_lists.tsv.zip", File: "camel-msa-top.tsv", ZipEntry: "MSA_freq_lists.tsv", MaxLines: 200000},
}

type locked struct {
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func fetch(ctx context.Context, client *http.Client, s source, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "mufradat-mkranking")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", s.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s", s.Name, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".fetch-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("downloading %s: %w", s.Name, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if s.ZipEntry == "" {
		if err := os.Chmod(tmp.Name(), 0o644); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), dest)
	}
	return extract(tmp.Name(), s, dest)
}

func extract(archive string, s source, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("opening the %s archive: %w", s.Name, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if filepath.Base(f.Name) != s.ZipEntry {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		out, err := os.CreateTemp(filepath.Dir(dest), ".extract-*")
		if err != nil {
			return err
		}
		defer os.Remove(out.Name())
		w := bufio.NewWriter(out)
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for lines := 0; sc.Scan() && (s.MaxLines == 0 || lines < s.MaxLines); lines++ {
			w.WriteString(sc.Text())
			w.WriteByte('\n')
		}
		if err := sc.Err(); err != nil {
			out.Close()
			return err
		}
		if err := w.Flush(); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		if err := os.Chmod(out.Name(), 0o644); err != nil {
			return err
		}
		return os.Rename(out.Name(), dest)
	}
	return fmt.Errorf("the %s archive has no %s", s.Name, s.ZipEntry)
}

func checksum(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func lockOf(name, url, path string) (locked, error) {
	sum, n, err := checksum(path)
	if err != nil {
		return locked{}, err
	}
	return locked{Name: name, URL: url, File: filepath.Base(path), SHA256: sum, Bytes: n}, nil
}

func readLock(path string) ([]locked, error) {
	if !fsutil.Exists(path) {
		return nil, nil
	}
	return jsonl.ReadFile[locked](path)
}
