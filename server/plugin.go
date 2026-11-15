package main

import (
	"context"
	"sync/atomic"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	client *pluginapi.Client
	store  *store

	// stopCtx is cancelled on deactivation so a running backfill stops between pages.
	stopCtx  context.Context
	stopFunc context.CancelFunc

	backfillRunning atomic.Bool
}

// OnActivate creates or migrates the link table, registers the slash commands and resumes the backfill.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)
	if driver := p.client.Store.DriverName(); driver != model.DatabaseDriverPostgres {
		return errors.Errorf("URL Search needs PostgreSQL, but this server uses %q", driver)
	}
	db, err := p.client.Store.GetMasterDB()
	if err != nil {
		return errors.Wrap(err, "open database")
	}
	p.store = &store{db: db}

	mutex, err := cluster.NewMutex(p.API, "migrate_mutex")
	if err != nil {
		return errors.Wrap(err, "create migration mutex")
	}
	mutex.Lock()
	err = p.store.migrate()
	mutex.Unlock()
	if err != nil {
		return errors.Wrap(err, "migrate link table")
	}

	for _, cmd := range commands() {
		if err := p.API.RegisterCommand(cmd); err != nil {
			return errors.Wrapf(err, "register /%s", cmd.Trigger)
		}
	}

	p.stopCtx, p.stopFunc = context.WithCancel(context.Background())
	if _, err := p.startBackfill(false); err != nil {
		return errors.Wrap(err, "start backfill")
	}
	return nil
}

// OnDeactivate stops the backfill after its current page and closes the database handle.
func (p *Plugin) OnDeactivate() error {
	if p.stopFunc != nil {
		p.stopFunc()
	}
	if p.client != nil {
		return p.client.Store.Close()
	}
	return nil
}
