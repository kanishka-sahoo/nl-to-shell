package context

import (
	"context"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// Gatherer implements the ContextGatherer interface
type Gatherer struct {
	plugins []interfaces.ContextPlugin
}

// NewGatherer creates a new context gatherer
func NewGatherer() interfaces.ContextGatherer {
	return &Gatherer{
		plugins: make([]interfaces.ContextPlugin, 0),
	}
}

// GatherContext collects environmental context information
func (g *Gatherer) GatherContext(ctx context.Context) (*types.Context, error) {
	// Implementation will be added in later tasks
	return &types.Context{}, nil
}

// RegisterPlugin registers a context plugin
func (g *Gatherer) RegisterPlugin(plugin interfaces.ContextPlugin) error {
	// Implementation will be added in later tasks
	g.plugins = append(g.plugins, plugin)
	return nil
}
