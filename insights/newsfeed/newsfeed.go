// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package newsfeed

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/rss"
	"github.com/rs/zerolog/log"
	"gitlab.com/lightmeter/controlcenter/insights/core"
	notificationCore "gitlab.com/lightmeter/controlcenter/notification/core"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"sort"
	"time"
)

type Options struct {
	URL            string
	UpdateInterval time.Duration
	RetryTime      time.Duration
	TimeLimit      time.Duration
}

type rssTranslator struct {
	defaultTranslator *gofeed.DefaultRSSTranslator
}

func (t *rssTranslator) Translate(feed interface{}) (*gofeed.Feed, error) {
	rss, found := feed.(*rss.Feed)
	if !found {
		return nil, errors.New(`Invalid feed format. Expected RSS`)
	}

	f, err := t.defaultTranslator.Translate(rss)
	if err != nil {
		return nil, errorutil.Wrap(err)
	}

	if len(f.Items) == 0 {
		return nil, errors.New(`Invalid feed. No items found`)
	}

	for i, item := range f.Items {
		desc, err := descForItem(item)

		if err != nil {
			return nil, errorutil.Wrap(err)
		}

		f.Items[i].Description = desc
	}

	return f, nil
}

func descForItem(item *gofeed.Item) (string, error) {
	lm, ok := item.Extensions["lightmeter"]
	if !ok {
		log.Warn().Msgf("Failed obtaining custom description for RSS item %s", item.GUID)
		return item.Description, nil
	}

	desc, ok := lm["newsInsightDescription"]
	if !ok {
		return "", errors.New(`Invalid feed. No custom description`)
	}

	if len(desc) == 0 {
		return "", errors.New(`Invalid feed. No description found`)
	}

	return desc[0].Value, nil
}

type detector struct {
	creator core.Creator
	options Options
	parser  *gofeed.Parser
}

func (*detector) Close() error {
	return nil
}

func NewDetector(creator core.Creator, options core.Options) core.Detector {
	detectorOptions, ok := options["newsfeed"].(Options)

	if !ok {
		errorutil.MustSucceed(errors.New("Invalid detector options"))
	}

	parser := gofeed.NewParser()

	parser.RSSTranslator = &rssTranslator{defaultTranslator: &gofeed.DefaultRSSTranslator{}}

	return &detector{creator: creator, options: detectorOptions, parser: parser}
}

// TODO: refactor this function to be reused across different insights instead of copy&pasted
func generateInsight(tx *sql.Tx, c core.Clock, creator core.Creator, content Content) error {
	properties := core.InsightProperties{
		Time:        c.Now(),
		Category:    core.NewsCategory,
		Rating:      core.Unrated,
		ContentType: ContentType,
		Content:     content,
	}

	if err := creator.GenerateInsight(context.Background(), tx, properties); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

type title string

func (t title) String() string {
	return string(t)
}

func (t title) TplString() string {
	return "%s"
}

func (t title) Args() []interface{} {
	return []interface{}{string(t)}
}

type description string

func (d description) String() string {
	return string(d)
}

func (d description) TplString() string {
	return "%s"
}

func (d description) Args() []interface{} {
	return []interface{}{string(d)}
}

type Content struct {
	TitleValue       title       `json:"title"`
	DescriptionValue description `json:"description"`
	Link             string      `json:"link"`
	Published        time.Time   `json:"date_published"`
	GUID             string      `json:"guid"`
}

func (c Content) Title() notificationCore.ContentComponent {
	return c.TitleValue
}

func (c Content) Description() notificationCore.ContentComponent {
	return c.DescriptionValue
}

func (c Content) Metadata() notificationCore.ContentMetadata {
	return nil
}

const kind = "newsfeed_last_exec"

func (d *detector) Step(c core.Clock, tx *sql.Tx) error {
	now := c.Now()

	lastExecTime, err := core.RetrieveLastDetectorExecution(tx, kind)
	if err != nil {
		return errorutil.Wrap(err)
	}

	if !lastExecTime.IsZero() && lastExecTime.Add(d.options.UpdateInterval).After(now) {
		return nil
	}

	timeout := time.Second * 3

	context, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Info().Msgf("Fetching news insights from %s", d.options.URL)

	parsed, err := d.parser.ParseURLWithContext(d.options.URL, context)
	if err != nil {
		log.Warn().Msgf("Failed to request newfeed insight source %s with error: %v", d.options.URL, err)

		// The request failed. Then try in the next time again
		if err := core.StoreLastDetectorExecution(tx, kind, now.Add(d.options.RetryTime).Add(-d.options.UpdateInterval)); err != nil {
			return errorutil.Wrap(err)
		}

		return nil
	}

	// sort by published time
	sort.Sort(parsed)

	for _, item := range parsed.Items {
		if item.PublishedParsed == nil {
			continue
		}

		timeLimit := now.Add(-d.options.TimeLimit)

		if item.PublishedParsed.Before(timeLimit) {
			// If the item is too old, do not consider it
			continue
		}

		alreadyExists, err := insightAlreadyExists(context, tx, item.GUID, timeLimit)
		if err != nil {
			return errorutil.Wrap(err)
		}

		if alreadyExists {
			continue
		}

		if err := generateInsight(tx, c, d.creator, Content{
			TitleValue:       title(item.Title),
			DescriptionValue: description(item.Description),
			Link:             item.Link,
			Published:        *item.PublishedParsed,
			GUID:             item.GUID,
		}); err != nil {
			return errorutil.Wrap(err)
		}
	}

	if err := core.StoreLastDetectorExecution(tx, kind, now); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

// rowserrcheck is not able to notice that query.Err() is called and emits a false positive warning
//nolint:rowserrcheck
func insightAlreadyExists(context context.Context, tx *sql.Tx, guid string, timeLimit time.Time) (bool, error) {
	rows, err := tx.QueryContext(context, `select content from insights where content_type = ? and time >= ?`, ContentTypeId, timeLimit.Unix())
	if err != nil {
		return false, errorutil.Wrap(err)
	}

	defer func() {
		errorutil.MustSucceed(rows.Close())
	}()

	for rows.Next() {
		var rawContent string

		err = rows.Scan(&rawContent)
		if err != nil {
			return false, errorutil.Wrap(err)
		}

		var content Content

		err = json.Unmarshal([]byte(rawContent), &content)
		if err != nil {
			return false, errorutil.Wrap(err)
		}

		if guid == content.GUID {
			return true, nil
		}
	}

	err = rows.Err()
	if err != nil {
		return false, errorutil.Wrap(err)
	}

	return false, nil
}

const (
	ContentType   = "newsfeed_content"
	ContentTypeId = 6
)

func init() {
	core.RegisterContentType(ContentType, ContentTypeId, core.DefaultContentTypeDecoder(&Content{}))
}
