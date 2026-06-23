package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/project"
	"github.com/alef-mach/tessera/internal/session"
)

type MemoryStore struct {
	path string
	db   *sql.DB
}

func NewMemoryStore(path string) *MemoryStore {
	return &MemoryStore{path: path}
}

func (s *MemoryStore) Ensure(ctx context.Context) error {
	if s.path == "" {
		return errors.New("sqlite memory path is required")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return err
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return err
	}
	s.db = db
	return nil
}

func (s *MemoryStore) SaveSession(ctx context.Context, sess session.Session) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, cwd, provider, model, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			cwd = excluded.cwd,
			provider = excluded.provider,
			model = excluded.model,
			updated_at = excluded.updated_at
	`, sess.ID, sess.CWD, sess.Provider, sess.Model, sess.CreatedAt, sess.UpdatedAt)
	return err
}

func (s *MemoryStore) GetSession(ctx context.Context, sessionID string) (session.Session, error) {
	if err := s.ready(); err != nil {
		return session.Session{}, err
	}
	query := `SELECT id, cwd, provider, model, created_at, updated_at FROM sessions`
	args := []any{}
	if sessionID != "" {
		query += ` WHERE id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY created_at DESC LIMIT 1`
	return scanSession(s.db.QueryRowContext(ctx, query, args...))
}

func (s *MemoryStore) ListSessions(ctx context.Context) ([]session.Session, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, cwd, provider, model, created_at, updated_at FROM sessions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []session.Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

func (s *MemoryStore) SaveRun(ctx context.Context, run memory.Run) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (id, session_id, input, status, steps, calls, started_at, updated_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			input = excluded.input,
			status = excluded.status,
			steps = excluded.steps,
			calls = excluded.calls,
			updated_at = excluded.updated_at,
			ended_at = excluded.ended_at
	`, run.ID, run.SessionID, run.Input, run.Status, run.Steps, run.Calls, run.StartedAt, run.UpdatedAt, run.EndedAt)
	return err
}

func (s *MemoryStore) GetRun(ctx context.Context, runID string) (memory.Run, error) {
	if err := s.ready(); err != nil {
		return memory.Run{}, err
	}
	return scanRun(s.db.QueryRowContext(ctx, `SELECT id, session_id, input, status, steps, calls, started_at, updated_at, ended_at FROM runs WHERE id = ?`, runID))
}

func (s *MemoryStore) ListRuns(ctx context.Context, sessionID string) ([]memory.Run, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, input, status, steps, calls, started_at, updated_at, ended_at FROM runs WHERE session_id = ? ORDER BY started_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []memory.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *MemoryStore) SaveCall(ctx context.Context, call memory.LLMCall) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO llm_calls (id, session_id, run_id, provider, model, prompt, system, response, input_tokens, output_tokens, duration_ms, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			response = excluded.response,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			duration_ms = excluded.duration_ms,
			error = excluded.error
	`, call.ID, call.SessionID, nullString(call.RunID), call.Provider, call.Model, call.Prompt, call.System, call.Response, call.InputTokens, call.OutputTokens, call.DurationMS, call.Error, call.CreatedAt)
	return err
}

func (s *MemoryStore) GetCall(ctx context.Context, callID string) (memory.LLMCall, error) {
	if err := s.ready(); err != nil {
		return memory.LLMCall{}, err
	}
	return scanCall(s.db.QueryRowContext(ctx, `SELECT id, session_id, run_id, provider, model, prompt, system, response, input_tokens, output_tokens, duration_ms, error, created_at FROM llm_calls WHERE id = ?`, callID))
}

func (s *MemoryStore) ListCalls(ctx context.Context, sessionID string) ([]memory.LLMCall, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, run_id, provider, model, prompt, system, response, input_tokens, output_tokens, duration_ms, error, created_at FROM llm_calls WHERE session_id = ? ORDER BY created_at DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []memory.LLMCall
	for rows.Next() {
		call, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, rows.Err()
}

func (s *MemoryStore) SaveObservation(ctx context.Context, observation memory.Observation) error {
	if err := s.ready(); err != nil {
		return err
	}
	data, err := json.Marshal(observation.Data)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO observations (id, session_id, run_id, kind, content, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind = excluded.kind,
			content = excluded.content,
			data = excluded.data
	`, observation.ID, observation.SessionID, nullString(observation.RunID), observation.Kind, observation.Content, string(data), observation.CreatedAt)
	return err
}

func (s *MemoryStore) GetObservation(ctx context.Context, observationID string) (memory.Observation, error) {
	if err := s.ready(); err != nil {
		return memory.Observation{}, err
	}
	return scanObservation(s.db.QueryRowContext(ctx, `SELECT id, session_id, run_id, kind, content, data, created_at FROM observations WHERE id = ?`, observationID))
}

func (s *MemoryStore) ListObservations(ctx context.Context, sessionID string) ([]memory.Observation, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, run_id, kind, content, data, created_at FROM observations WHERE session_id = ? ORDER BY created_at DESC, id DESC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []memory.Observation
	for rows.Next() {
		observation, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	return observations, rows.Err()
}

func (s *MemoryStore) SaveFileSummary(ctx context.Context, summary memory.FileSummary) error {
	if err := s.ready(); err != nil {
		return err
	}
	imports, err := json.Marshal(summary.Imports)
	if err != nil {
		return err
	}
	exports, err := json.Marshal(summary.Exports)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO file_summaries (id, session_id, path, language, summary, hash, imports, exports, has_tests_nearby, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, path) DO UPDATE SET
			language = excluded.language,
			summary = excluded.summary,
			hash = excluded.hash,
			imports = excluded.imports,
			exports = excluded.exports,
			has_tests_nearby = excluded.has_tests_nearby,
			updated_at = excluded.updated_at
	`, summary.ID, summary.SessionID, summary.Path, summary.Language, summary.Summary, summary.Hash, string(imports), string(exports), summary.HasTestsNearby, summary.UpdatedAt)
	return err
}

func (s *MemoryStore) GetFileSummary(ctx context.Context, sessionID, path string) (memory.FileSummary, error) {
	if err := s.ready(); err != nil {
		return memory.FileSummary{}, err
	}
	return scanFileSummary(s.db.QueryRowContext(ctx, `SELECT id, session_id, path, language, summary, hash, imports, exports, has_tests_nearby, updated_at FROM file_summaries WHERE session_id = ? AND path = ?`, sessionID, path))
}

func (s *MemoryStore) ListFileSummaries(ctx context.Context, sessionID string) ([]memory.FileSummary, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, path, language, summary, hash, imports, exports, has_tests_nearby, updated_at FROM file_summaries WHERE session_id = ? ORDER BY path`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []memory.FileSummary
	for rows.Next() {
		summary, err := scanFileSummary(rows)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (s *MemoryStore) SaveSymbol(ctx context.Context, symbol memory.Symbol) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO symbols (id, session_id, name, kind, path, line, start_line, end_line, summary, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			kind = excluded.kind,
			path = excluded.path,
			line = excluded.line,
			start_line = excluded.start_line,
			end_line = excluded.end_line,
			summary = excluded.summary,
			updated_at = excluded.updated_at
	`, symbol.ID, symbol.SessionID, symbol.Name, symbol.Kind, symbol.Path, symbol.Line, symbol.StartLine, symbol.EndLine, symbol.Summary, symbol.UpdatedAt)
	return err
}

func (s *MemoryStore) GetSymbol(ctx context.Context, symbolID string) (memory.Symbol, error) {
	if err := s.ready(); err != nil {
		return memory.Symbol{}, err
	}
	return scanSymbol(s.db.QueryRowContext(ctx, `SELECT id, session_id, name, kind, path, line, start_line, end_line, summary, updated_at FROM symbols WHERE id = ?`, symbolID))
}

func (s *MemoryStore) ListSymbols(ctx context.Context, sessionID string) ([]memory.Symbol, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, session_id, name, kind, path, line, start_line, end_line, summary, updated_at FROM symbols WHERE session_id = ? ORDER BY path, line, name`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []memory.Symbol
	for rows.Next() {
		symbol, err := scanSymbol(rows)
		if err != nil {
			return nil, err
		}
		symbols = append(symbols, symbol)
	}
	return symbols, rows.Err()
}

func (s *MemoryStore) ClearIndex(ctx context.Context, sessionID string) error {
	if err := s.ready(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM symbols WHERE session_id = ?`, sessionID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM file_summaries WHERE session_id = ?`, sessionID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *MemoryStore) SaveProjectProfile(ctx context.Context, profile project.ProjectProfile) error {
	if err := s.ready(); err != nil {
		return err
	}
	manifests, err := json.Marshal(profile.Manifests)
	if err != nil {
		return err
	}
	stacks, err := json.Marshal(profile.Stacks)
	if err != nil {
		return err
	}
	testPaths, err := json.Marshal(profile.TestPaths)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO project_profiles (session_id, root, mode, stack, stacks, manifests, has_git, has_tests, test_paths, test_runner, profiled_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			root = excluded.root,
			mode = excluded.mode,
			stack = excluded.stack,
			stacks = excluded.stacks,
			manifests = excluded.manifests,
			has_git = excluded.has_git,
			has_tests = excluded.has_tests,
			test_paths = excluded.test_paths,
			test_runner = excluded.test_runner,
			profiled_at = excluded.profiled_at
	`, profile.SessionID, profile.Root, profile.Mode, profile.Stack, string(stacks), string(manifests), profile.HasGit, profile.HasTests, string(testPaths), profile.TestRunner, profile.ProfiledAt)
	return err
}

func (s *MemoryStore) GetProjectProfile(ctx context.Context, sessionID string) (project.ProjectProfile, error) {
	if err := s.ready(); err != nil {
		return project.ProjectProfile{}, err
	}
	return scanProjectProfile(s.db.QueryRowContext(ctx, `
		SELECT session_id, root, mode, stack, stacks, manifests, has_git, has_tests, test_paths, test_runner, profiled_at
		FROM project_profiles
		WHERE session_id = ?
	`, sessionID))
}

func (s *MemoryStore) SaveEvent(ctx context.Context, sessionID string, evt event.Event) error {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = memoryTimestamp()
	}
	content := evt.Title
	if evt.Message != "" {
		content = evt.Title + "\n" + evt.Message
	}
	return s.SaveObservation(ctx, memory.Observation{
		ID:        "evt-" + evt.Timestamp.UTC().Format("20060102-150405.000000000"),
		SessionID: sessionID,
		Kind:      "event",
		Content:   content,
		Data: map[string]any{
			"type":    evt.Type,
			"title":   evt.Title,
			"message": evt.Message,
			"data":    evt.Data,
		},
		CreatedAt: evt.Timestamp,
	})
}

func (s *MemoryStore) ListEvents(ctx context.Context, sessionID string) ([]event.Event, error) {
	observations, err := s.ListObservations(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	events := make([]event.Event, 0, len(observations))
	for _, observation := range observations {
		if observation.Kind != "event" {
			continue
		}
		events = append(events, observationToEvent(observation))
	}
	return events, nil
}

func (s *MemoryStore) Stats(ctx context.Context, sessionID string) (memory.Stats, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return memory.Stats{}, err
	}
	stats := memory.Stats{SessionID: sess.ID, Provider: sess.Provider, Model: sess.Model}
	counts := map[string]*int{
		`SELECT COUNT(*) FROM llm_calls WHERE session_id = ?`:           &stats.Calls,
		`SELECT COALESCE(SUM(steps), 0) FROM runs WHERE session_id = ?`: &stats.Steps,
		`SELECT COUNT(*) FROM runs WHERE session_id = ?`:                &stats.Runs,
		`SELECT COUNT(*) FROM observations WHERE session_id = ?`:        &stats.Observations,
		`SELECT COUNT(*) FROM file_summaries WHERE session_id = ?`:      &stats.FileSummaries,
		`SELECT COUNT(*) FROM symbols WHERE session_id = ?`:             &stats.Symbols,
	}
	for query, target := range counts {
		if err := s.db.QueryRowContext(ctx, query, sess.ID).Scan(target); err != nil {
			return memory.Stats{}, err
		}
	}
	return stats, nil
}

func (s *MemoryStore) ready() error {
	if s.db == nil {
		return errors.New("sqlite memory store is not initialized")
	}
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			cwd TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			input TEXT NOT NULL,
			status TEXT NOT NULL,
			steps INTEGER NOT NULL DEFAULT 0,
			calls INTEGER NOT NULL DEFAULT 0,
			started_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			ended_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS llm_calls (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			run_id TEXT REFERENCES runs(id) ON DELETE SET NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			prompt TEXT NOT NULL,
			system TEXT NOT NULL,
			response TEXT NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS observations (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			run_id TEXT REFERENCES runs(id) ON DELETE SET NULL,
			kind TEXT NOT NULL,
			content TEXT NOT NULL,
			data TEXT NOT NULL DEFAULT '{}',
			created_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS file_summaries (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			language TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL,
			hash TEXT NOT NULL,
			imports TEXT NOT NULL DEFAULT '[]',
			exports TEXT NOT NULL DEFAULT '[]',
			has_tests_nearby BOOLEAN NOT NULL DEFAULT 0,
			updated_at TIMESTAMP NOT NULL,
			UNIQUE(session_id, path)
		)`,
		`CREATE TABLE IF NOT EXISTS symbols (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			path TEXT NOT NULL,
			line INTEGER NOT NULL DEFAULT 0,
			start_line INTEGER NOT NULL DEFAULT 0,
			end_line INTEGER NOT NULL DEFAULT 0,
			summary TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_profiles (
			session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
			root TEXT NOT NULL,
			mode TEXT NOT NULL,
			stack TEXT NOT NULL,
			stacks TEXT NOT NULL DEFAULT '[]',
			manifests TEXT NOT NULL DEFAULT '[]',
			has_git BOOLEAN NOT NULL DEFAULT 0,
			has_tests BOOLEAN NOT NULL DEFAULT 0,
			test_paths TEXT NOT NULL DEFAULT '[]',
			test_runner TEXT NOT NULL,
			profiled_at TIMESTAMP NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := ensureColumn(ctx, db, "file_summaries", "language", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, db, "file_summaries", "imports", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, db, "file_summaries", "exports", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, db, "file_summaries", "has_tests_nearby", "BOOLEAN NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, db, "symbols", "start_line", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(ctx, db, "symbols", "end_line", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (session.Session, error) {
	var sess session.Session
	err := row.Scan(&sess.ID, &sess.CWD, &sess.Provider, &sess.Model, &sess.CreatedAt, &sess.UpdatedAt)
	return sess, err
}

func scanRun(row rowScanner) (memory.Run, error) {
	var run memory.Run
	err := row.Scan(&run.ID, &run.SessionID, &run.Input, &run.Status, &run.Steps, &run.Calls, &run.StartedAt, &run.UpdatedAt, &run.EndedAt)
	return run, err
}

func scanCall(row rowScanner) (memory.LLMCall, error) {
	var call memory.LLMCall
	var runID sql.NullString
	err := row.Scan(&call.ID, &call.SessionID, &runID, &call.Provider, &call.Model, &call.Prompt, &call.System, &call.Response, &call.InputTokens, &call.OutputTokens, &call.DurationMS, &call.Error, &call.CreatedAt)
	call.RunID = runID.String
	return call, err
}

func scanObservation(row rowScanner) (memory.Observation, error) {
	var observation memory.Observation
	var runID sql.NullString
	var rawData string
	err := row.Scan(&observation.ID, &observation.SessionID, &runID, &observation.Kind, &observation.Content, &rawData, &observation.CreatedAt)
	if err != nil {
		return memory.Observation{}, err
	}
	observation.RunID = runID.String
	if rawData == "" {
		rawData = "{}"
	}
	if err := json.Unmarshal([]byte(rawData), &observation.Data); err != nil {
		return memory.Observation{}, err
	}
	return observation, nil
}

func scanFileSummary(row rowScanner) (memory.FileSummary, error) {
	var summary memory.FileSummary
	var imports, exports string
	err := row.Scan(&summary.ID, &summary.SessionID, &summary.Path, &summary.Language, &summary.Summary, &summary.Hash, &imports, &exports, &summary.HasTestsNearby, &summary.UpdatedAt)
	if err != nil {
		return memory.FileSummary{}, err
	}
	if imports == "" {
		imports = "[]"
	}
	if exports == "" {
		exports = "[]"
	}
	if err := json.Unmarshal([]byte(imports), &summary.Imports); err != nil {
		return memory.FileSummary{}, err
	}
	if err := json.Unmarshal([]byte(exports), &summary.Exports); err != nil {
		return memory.FileSummary{}, err
	}
	return summary, err
}

func scanSymbol(row rowScanner) (memory.Symbol, error) {
	var symbol memory.Symbol
	err := row.Scan(&symbol.ID, &symbol.SessionID, &symbol.Name, &symbol.Kind, &symbol.Path, &symbol.Line, &symbol.StartLine, &symbol.EndLine, &symbol.Summary, &symbol.UpdatedAt)
	if symbol.StartLine == 0 {
		symbol.StartLine = symbol.Line
	}
	if symbol.EndLine == 0 {
		symbol.EndLine = symbol.Line
	}
	return symbol, err
}

func scanProjectProfile(row rowScanner) (project.ProjectProfile, error) {
	var profile project.ProjectProfile
	var stacks, manifests, testPaths string
	err := row.Scan(&profile.SessionID, &profile.Root, &profile.Mode, &profile.Stack, &stacks, &manifests, &profile.HasGit, &profile.HasTests, &testPaths, &profile.TestRunner, &profile.ProfiledAt)
	if err != nil {
		return project.ProjectProfile{}, err
	}
	if err := json.Unmarshal([]byte(stacks), &profile.Stacks); err != nil {
		return project.ProjectProfile{}, err
	}
	if err := json.Unmarshal([]byte(manifests), &profile.Manifests); err != nil {
		return project.ProjectProfile{}, err
	}
	if err := json.Unmarshal([]byte(testPaths), &profile.TestPaths); err != nil {
		return project.ProjectProfile{}, err
	}
	return profile, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func observationToEvent(observation memory.Observation) event.Event {
	evt := event.Event{Timestamp: observation.CreatedAt}
	evt.Type, _ = observation.Data["type"].(string)
	evt.Title, _ = observation.Data["title"].(string)
	evt.Message, _ = observation.Data["message"].(string)
	if data, ok := observation.Data["data"].(map[string]any); ok {
		evt.Data = data
	}
	if evt.Type == "" {
		evt.Type = observation.Kind
	}
	if evt.Title == "" {
		evt.Title = observation.Content
	}
	return evt
}
