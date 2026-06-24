package main

import (
	"fmt"
	"os"

	"github.com/alef-mach/tessera/internal/event"
	"github.com/alef-mach/tessera/internal/trust"
	"github.com/alef-mach/tessera/internal/ui/plain"
)

func ensureTrustedFolder(ui *plain.Renderer) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}

	store, err := trust.NewStore()
	if err != nil {
		return false, err
	}

	trusted, err := store.IsTrusted(cwd)
	if err != nil {
		return false, err
	}
	if trusted {
		return true, nil
	}

	if !askToTrustFolder(ui, store, cwd) {
		ui.RenderEvent(event.New(
			"workspace.trust.denied",
			"Folder not trusted",
			"Session cancelled.",
			map[string]any{"cwd": cwd},
		))
		return false, nil
	}

	if err := store.Trust(cwd); err != nil {
		return false, err
	}

	ui.RenderEvent(event.New(
		"workspace.trust.saved",
		"Folder trusted",
		cwd,
		map[string]any{
			"cwd":         cwd,
			"trust_store": store.Path(),
		},
	))

	return true, nil
}

func askToTrustFolder(ui *plain.Renderer, store trust.Store, cwd string) bool {
	return ui.AskApproval(event.New(
		"workspace.trust.requested",
		"Trust this folder?",
		fmt.Sprintf(
			"Tessera will be allowed to create local session data and operate in:\n%s",
			cwd,
		),
		map[string]any{
			"cwd":         cwd,
			"trust_store": store.Path(),
		},
	))
}
