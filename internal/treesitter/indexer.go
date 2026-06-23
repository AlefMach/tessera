package treesitter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alef-mach/tessera/internal/treesitter/fallback"
	"github.com/alef-mach/tessera/internal/treesitter/languages"
	"github.com/alef-mach/tessera/internal/treesitter/model"
	"github.com/alef-mach/tessera/internal/treesitter/parser"
	"github.com/alef-mach/tessera/internal/treesitter/persist"
	"github.com/alef-mach/tessera/internal/treesitter/repomap"
	"github.com/alef-mach/tessera/internal/treesitter/scan"
)

func (i *Indexer) Index(ctx context.Context, sessionID string) (Result, error) {
	if i.store == nil {
		return Result{}, errors.New("index store is required")
	}
	started := i.now()
	files, err := scan.SourceFiles(i.root, languages.IndexableExtensions())
	if err != nil {
		return Result{}, err
	}
	if err := i.store.ClearIndex(ctx, sessionID); err != nil {
		return Result{}, err
	}

	result := Result{StartedAt: started}
	for _, path := range files {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}

		index, hash, err := i.indexFile(path)
		if err != nil {
			continue
		}
		if err := i.saveFile(ctx, sessionID, index, hash); err != nil {
			return Result{}, err
		}
		result.Files++
		result.Symbols += len(index.Symbols)
		result.Summaries = append(result.Summaries, index)
	}

	result.FinishedAt = i.now()
	result.RepoMap = RepoMap(result.Summaries)
	return result, nil
}

func (i *Indexer) indexFile(absPath string) (model.FileIndex, string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return model.FileIndex{}, "", err
	}
	if len(data) > maxIndexedFileBytes {
		return model.FileIndex{}, "", fmt.Errorf("file too large: %s", absPath)
	}

	rel, err := filepath.Rel(i.root, absPath)
	if err != nil {
		return model.FileIndex{}, "", err
	}
	rel = filepath.ToSlash(rel)
	spec := languages.SpecForPath(rel)

	index := model.FileIndex{
		Path:           rel,
		Language:       spec.Name,
		HasTestsNearby: scan.HasTestsNearby(i.root, rel),
	}
	hashBytes := sha256.Sum256(data)
	hash := hex.EncodeToString(hashBytes[:])

	if spec.Parser != nil {
		parsed, err := parser.Parse(data, spec)
		if err == nil {
			index.Symbols = parsed.Symbols
			index.Imports = parsed.Imports
			index.Exports = parsed.Exports
			return repomap.Normalize(index), hash, nil
		}
	}

	index.Symbols, index.Imports, index.Exports = fallback.RegexIndex(data, spec.Regex)
	index.Fallback = "regex"
	if len(index.Symbols) == 0 {
		index.Symbols = fallback.LineChunks(data)
		index.Fallback = "line_chunks"
	}
	return repomap.Normalize(index), hash, nil
}

func (i *Indexer) saveFile(ctx context.Context, sessionID string, index model.FileIndex, hash string) error {
	encoded, err := json.Marshal(index)
	if err != nil {
		return err
	}
	now := i.now()
	if err := i.store.SaveFileSummary(ctx, persist.FileSummary(sessionID, index, hash, string(encoded), now)); err != nil {
		return err
	}
	for _, symbol := range index.Symbols {
		summary, err := json.Marshal(symbol)
		if err != nil {
			return err
		}
		if err := i.store.SaveSymbol(ctx, persist.Symbol(sessionID, index.Path, symbol, string(summary), now)); err != nil {
			return err
		}
	}
	return nil
}

func RepoMap(files []model.FileIndex) string {
	return repomap.Build(files)
}
