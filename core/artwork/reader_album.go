package artwork

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/maruel/natural"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/external"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
	"golang.org/x/crypto/sha3"
)

type albumArtworkReader struct {
	cacheKey
	a          *artwork
	provider   external.Provider
	album      model.Album
	updatedAt  *time.Time
	imgFiles   []string
	rootFolder string
}

func newAlbumArtworkReader(ctx context.Context, artwork *artwork, artID model.ArtworkID, provider external.Provider) (*albumArtworkReader, error) {
	al, err := artwork.ds.Album(ctx).Get(artID.ID)
	if err != nil {
		return nil, err
	}
	_, imgFiles, imagesUpdateAt, err := loadAlbumFoldersPaths(ctx, artwork.ds, *al)
	if err != nil {
		return nil, err
	}
	a := &albumArtworkReader{
		a:          artwork,
		provider:   provider,
		album:      *al,
		updatedAt:  imagesUpdateAt,
		imgFiles:   imgFiles,
		rootFolder: core.AbsolutePath(ctx, artwork.ds, al.LibraryID, ""),
	}
	a.cacheKey.artID = artID
	if a.updatedAt != nil && a.updatedAt.After(al.UpdatedAt) {
		a.cacheKey.lastUpdate = *a.updatedAt
	} else {
		a.cacheKey.lastUpdate = al.UpdatedAt
	}
	return a, nil
}

// Key returns a cache key for the album artwork
// Uses SHA3-256 (post-quantum resistant) for hash generation
// Version 2: Added fallback image support - cache key includes version to invalidate old entries
const artworkCacheVersion = "v2"

func (a *albumArtworkReader) Key() string {
	var hash [16]byte
	if conf.Server.EnableExternalServices {
		full := sha3.Sum256([]byte(conf.Server.Agents + conf.Server.CoverArtPriority))
		copy(hash[:], full[:16])
	}
	return fmt.Sprintf(
		"%s.%x.%t.%s",
		a.cacheKey.Key(),
		hash,
		conf.Server.EnableExternalServices,
		artworkCacheVersion,
	)
}
func (a *albumArtworkReader) LastUpdated() time.Time {
	return a.album.UpdatedAt
}

func (a *albumArtworkReader) Reader(ctx context.Context) (io.ReadCloser, string, error) {
	var ff = a.fromCoverArtPriority(ctx, a.a.ffmpeg, conf.Server.CoverArtPriority)
	return selectImageReader(ctx, a.artID, ff...)
}

func (a *albumArtworkReader) fromCoverArtPriority(ctx context.Context, ffmpeg ffmpeg.FFmpeg, priority string) []sourceFunc {
	var ff []sourceFunc
	for _, pattern := range strings.Split(strings.ToLower(priority), ",") {
		pattern = strings.TrimSpace(pattern)
		switch {
		case pattern == "embedded":
			embedArtPath := filepath.Join(a.rootFolder, a.album.EmbedArtPath)
			ff = append(ff, fromTag(ctx, embedArtPath), fromFFmpegTag(ctx, ffmpeg, embedArtPath))
		case pattern == "external":
			ff = append(ff, fromAlbumExternalSource(ctx, a.album, a.provider))
		case len(a.imgFiles) > 0:
			ff = append(ff, fromExternalFile(ctx, a.imgFiles, pattern))
		}
	}
	// Fallback: if no standard patterns matched and we have image files, try any image
	if len(a.imgFiles) > 0 {
		ff = append(ff, fromAnyImageFile(ctx, a.imgFiles))
	}
	return ff
}

func loadAlbumFoldersPaths(ctx context.Context, ds model.DataStore, albums ...model.Album) ([]string, []string, *time.Time, error) {
	var folderIDs []string
	for _, album := range albums {
		folderIDs = append(folderIDs, album.FolderIDs...)
	}
	folders, err := ds.Folder(ctx).GetAll(model.QueryOptions{Filters: squirrel.Eq{"folder.id": folderIDs, "missing": false}})
	if err != nil {
		return nil, nil, nil, err
	}
	var paths []string
	var imgFiles []string
	var updatedAt time.Time
	for _, f := range folders {
		path := f.AbsolutePath()
		paths = append(paths, path)
		if f.ImagesUpdatedAt.After(updatedAt) {
			updatedAt = f.ImagesUpdatedAt
		}
		for _, img := range f.ImageFiles {
			imgFiles = append(imgFiles, filepath.Join(path, img))
		}
	}

	// Sort image files to ensure consistent selection of cover art
	// This prioritizes files without numeric suffixes (e.g., cover.jpg over cover.1.jpg)
	// by comparing base filenames without extensions
	slices.SortFunc(imgFiles, compareImageFiles)

	return paths, imgFiles, &updatedAt, nil
}

// compareImageFiles compares two image file paths for sorting.
// It extracts the base filename (without extension) and compares case-insensitively.
// This ensures that "cover.jpg" sorts before "cover.1.jpg" since "cover" < "cover.1".
// Note: This function is called O(n log n) times during sorting, but in practice albums
// typically have only 1-20 image files, making the repeated string operations negligible.
func compareImageFiles(a, b string) int {
	// Case-insensitive comparison
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	// Extract base filenames without extensions
	baseA := strings.TrimSuffix(filepath.Base(a), filepath.Ext(a))
	baseB := strings.TrimSuffix(filepath.Base(b), filepath.Ext(b))

	// Compare base names first, then full paths if equal
	return cmp.Or(
		natural.Compare(baseA, baseB),
		natural.Compare(a, b),
	)
}
