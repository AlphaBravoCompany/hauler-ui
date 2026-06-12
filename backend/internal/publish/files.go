package publish

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ociManifest is the subset of an OCI image manifest we need to locate the
// content layer of a file/chart artifact.
type ociManifest struct {
	Layers []struct {
		MediaType   string            `json:"mediaType"`
		Size        int64             `json:"size"`
		Digest      string            `json:"digest"`
		Annotations map[string]string `json:"annotations"`
	} `json:"layers"`
}

// FileEntry is a downloadable artifact (file or chart) in a haul's store.
type FileEntry struct {
	Name   string `json:"name"` // original filename (layer title)
	Size   int64  `json:"size"`
	Digest string `json:"digest"` // content layer digest
}

// blobPath maps a "sha256:hex" digest to its on-disk blob path.
func blobPath(storeDir, digest string) string {
	hex := digest
	if i := strings.IndexByte(hex, ':'); i != -1 {
		hex = hex[i+1:]
	}
	return filepath.Join(storeDir, "blobs", "sha256", hex)
}

// readIndexManifestDigests returns the digests of the OCI manifests listed in a
// store's index.json.
func readIndexManifestDigests(storeDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(storeDir, "index.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var index struct {
		Manifests []struct {
			Digest string `json:"digest"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	digests := make([]string, 0, len(index.Manifests))
	for _, m := range index.Manifests {
		digests = append(digests, m.Digest)
	}
	return digests, nil
}

// listFiles enumerates downloadable artifacts (those whose content layer carries
// an org.opencontainers.image.title annotation, i.e. files and charts).
func listFiles(storeDir string) ([]FileEntry, error) {
	digests, err := readIndexManifestDigests(storeDir)
	if err != nil {
		return nil, err
	}
	var entries []FileEntry
	seen := map[string]bool{}
	for _, d := range digests {
		data, err := os.ReadFile(blobPath(storeDir, d))
		if err != nil {
			continue
		}
		var man ociManifest
		if err := json.Unmarshal(data, &man); err != nil {
			continue
		}
		for _, layer := range man.Layers {
			title := layer.Annotations["org.opencontainers.image.title"]
			if title == "" || seen[title] {
				continue
			}
			seen[title] = true
			entries = append(entries, FileEntry{Name: title, Size: layer.Size, Digest: layer.Digest})
		}
	}
	return entries, nil
}

// findFileLayer locates the content layer for a named artifact in a store.
func findFileLayer(storeDir, name string) (digest string, size int64, ok bool) {
	digests, err := readIndexManifestDigests(storeDir)
	if err != nil {
		return "", 0, false
	}
	for _, d := range digests {
		data, err := os.ReadFile(blobPath(storeDir, d))
		if err != nil {
			continue
		}
		var man ociManifest
		if err := json.Unmarshal(data, &man); err != nil {
			continue
		}
		for _, layer := range man.Layers {
			if layer.Annotations["org.opencontainers.image.title"] == name {
				return layer.Digest, layer.Size, true
			}
		}
	}
	return "", 0, false
}

// serveFileList writes the JSON listing of a haul's downloadable artifacts.
func serveFileList(w http.ResponseWriter, storeDir string) {
	entries, err := listFiles(storeDir)
	if err != nil {
		http.Error(w, "failed to read store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []FileEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"files": entries})
}

// serveFile streams a single named artifact's content blob with its filename.
func serveFile(w http.ResponseWriter, r *http.Request, storeDir, name string) {
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	digest, _, ok := findFileLayer(storeDir, name)
	if !ok {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	f, err := os.Open(blobPath(storeDir, digest))
	if err != nil {
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "stat failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Type", "application/octet-stream")
	// http.ServeContent gives us Range support for free.
	http.ServeContent(w, r, name, info.ModTime(), f)
}
