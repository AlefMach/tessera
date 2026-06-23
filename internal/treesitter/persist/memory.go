package persist

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/treesitter/model"
)

func FileSummary(sessionID string, index model.FileIndex, hash, summary string, now time.Time) memory.FileSummary {
	return memory.FileSummary{
		ID:             stableID(sessionID, index.Path),
		SessionID:      sessionID,
		Path:           index.Path,
		Language:       index.Language,
		Summary:        summary,
		Hash:           hash,
		Imports:        index.Imports,
		Exports:        index.Exports,
		HasTestsNearby: index.HasTestsNearby,
		UpdatedAt:      now,
	}
}

func Symbol(sessionID, path string, symbol model.SymbolIndex, summary string, now time.Time) memory.Symbol {
	line := symbol.StartLine
	if line == 0 {
		line = symbol.EndLine
	}
	return memory.Symbol{
		ID:        stableID(sessionID, path, symbol.Kind, symbol.Name, fmt.Sprint(symbol.StartLine), fmt.Sprint(symbol.EndLine)),
		SessionID: sessionID,
		Name:      symbol.Name,
		Kind:      symbol.Kind,
		Path:      path,
		Line:      line,
		StartLine: symbol.StartLine,
		EndLine:   symbol.EndLine,
		Summary:   summary,
		UpdatedAt: now,
	}
}

func stableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "idx-" + hex.EncodeToString(sum[:])[:24]
}
