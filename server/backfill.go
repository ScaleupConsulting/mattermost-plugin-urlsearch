package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"
)

const (
	backfillCursorKey = "backfill_cursor"
	backfillMutexKey  = "backfill_mutex"
	backfillPageSize  = 500
)

// startBackfill runs the backfill in the background unless it already finished. With restart it
// first resets the cursor so every post is reindexed. It returns false if a backfill is running.
func (p *Plugin) startBackfill(restart bool) (bool, error) {
	if !p.backfillRunning.CompareAndSwap(false, true) {
		return false, nil
	}
	if restart {
		if err := p.saveCursor(backfillCursor{}); err != nil {
			p.backfillRunning.Store(false)
			return false, err
		}
	}
	go func() {
		defer p.backfillRunning.Store(false)
		if err := p.runBackfill(); err != nil {
			p.API.LogError("Link backfill failed; it resumes from its cursor on the next run", "err", err.Error())
		}
	}()
	return true, nil
}

// runBackfill indexes existing posts page by page from the saved cursor, saving the cursor after
// each page so a restart or deactivation resumes where it stopped. The cluster mutex keeps it to
// one node at a time.
func (p *Plugin) runBackfill() error {
	mutex, err := cluster.NewMutex(p.API, backfillMutexKey)
	if err != nil {
		return errors.Wrap(err, "create backfill mutex")
	}
	lockCtx, cancel := context.WithTimeout(p.stopCtx, time.Second)
	defer cancel()
	if err := mutex.LockWithContext(lockCtx); err != nil {
		p.API.LogInfo("Link backfill is running on another node; not starting here")
		return nil
	}
	defer mutex.Unlock()

	cursor, err := p.loadCursor()
	if err != nil {
		return err
	}
	if cursor.Done {
		return nil
	}
	indexed := 0
	for p.stopCtx.Err() == nil {
		posts, err := p.store.nextBackfillPage(cursor, backfillPageSize)
		if err != nil {
			return err
		}
		for _, post := range posts {
			if err := p.store.replacePostLinks(post, extractLinks(post.Message)); err != nil {
				return errors.Wrapf(err, "index post %s", post.ID)
			}
			cursor = backfillCursor{CreateAt: post.CreateAt, PostID: post.ID}
			indexed++
		}
		cursor.Done = len(posts) < backfillPageSize
		if err := p.saveCursor(cursor); err != nil {
			return err
		}
		if cursor.Done {
			p.API.LogInfo("Link backfill complete", "posts_indexed_this_run", indexed)
			return nil
		}
	}
	p.API.LogInfo("Link backfill paused at deactivation", "posts_indexed_this_run", indexed)
	return nil
}

// loadCursor reads the backfill position; a missing key means start from the beginning.
func (p *Plugin) loadCursor() (backfillCursor, error) {
	var c backfillCursor
	data, appErr := p.API.KVGet(backfillCursorKey)
	if appErr != nil {
		return c, errors.Wrap(appErr, "read backfill cursor")
	}
	if data == nil {
		return c, nil
	}
	return c, errors.Wrap(json.Unmarshal(data, &c), "decode backfill cursor")
}

// saveCursor stores the backfill position.
func (p *Plugin) saveCursor(c backfillCursor) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if appErr := p.API.KVSet(backfillCursorKey, data); appErr != nil {
		return errors.Wrap(appErr, "save backfill cursor")
	}
	return nil
}
