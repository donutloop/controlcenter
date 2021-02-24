// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package insights

import (
	"context"
	"database/sql"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/lightmeter/controlcenter/data"
	"gitlab.com/lightmeter/controlcenter/i18n/translator"
	"gitlab.com/lightmeter/controlcenter/insights/core"
	insighttestsutil "gitlab.com/lightmeter/controlcenter/insights/testutil"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/dbconn"
	"gitlab.com/lightmeter/controlcenter/notification"
	notificationCore "gitlab.com/lightmeter/controlcenter/notification/core"
	"gitlab.com/lightmeter/controlcenter/util/testutil"
	"golang.org/x/text/language"
	"golang.org/x/text/message/catalog"
	"sync"
	"testing"
	"time"
)

var (
	dummyContext = context.Background()
)

func init() {
	lmsqlite3.Initialize(lmsqlite3.Options{})
}

type content struct {
	T string `json:"title"`
	D string `json:"description"`
}

func (c content) Title() notificationCore.ContentComponent {
	return fakeContentComponent(c.T)
}

func (c content) Description() notificationCore.ContentComponent {
	return fakeContentComponent(c.D)
}

func (c content) Metadata() notificationCore.ContentMetadata {
	return nil
}

type fakeContentComponent string

func (c fakeContentComponent) String() string {
	return translator.Stringfy(c)
}

func (c fakeContentComponent) TplString() string {
	return "%s"
}

func (c fakeContentComponent) Args() []interface{} {
	return []interface{}{c}
}

type fakeNotifier struct {
	notifications []notification.Notification
}

func (*fakeNotifier) ValidateSettings(notificationCore.Settings) error {
	return nil
}

func (f *fakeNotifier) Notify(n notification.Notification, _ translator.Translator) error {
	f.notifications = append(f.notifications, n)
	return nil
}

type fakeValue struct {
	Category core.Category
	Rating   core.Rating
	Content  core.Content
}

type fakeDetector struct {
	t *testing.T
	// added just to silent the race detector during tests
	sync.Mutex
	creator   *creator
	fakeValue *fakeValue
}

func (d *fakeDetector) value() *fakeValue {
	d.Lock()
	defer d.Unlock()
	return d.fakeValue
}

func (d *fakeDetector) setValue(v *fakeValue) {
	d.Lock()
	defer d.Unlock()
	d.fakeValue = v
}

func (*fakeDetector) Close() error {
	return nil
}

func (d *fakeDetector) Setup(*sql.Tx) error {
	return nil
}

func (d *fakeDetector) GenerateSampleInsight(tx *sql.Tx, clock core.Clock) error {
	return d.creator.GenerateInsight(tx, core.InsightProperties{
		Time:        clock.Now(),
		Category:    core.IntelCategory,
		ContentType: "fake_insight_type",
		Content:     &content{T: "title", D: "description"},
		Rating:      core.BadRating,
	})
}

func init() {
	core.RegisterContentType("fake_insight_type", 200, core.DefaultContentTypeDecoder(&content{}))
}

func (d *fakeDetector) Step(clock core.Clock, tx *sql.Tx) error {
	p := d.value()

	if p == nil {
		return nil
	}

	v := *p

	d.t.Log("New Fake Insight at time ", clock.Now())

	if err := d.creator.GenerateInsight(tx, core.InsightProperties{
		Time:        clock.Now(),
		Category:    v.Category,
		ContentType: "fake_insight_type",
		Content:     v.Content,
		Rating:      v.Rating,
	}); err != nil {
		return err
	}

	d.setValue(nil)

	return nil
}

func TestEngine(t *testing.T) {
	Convey("Test Insights Generator", t, func() {
		dir, clearDir := testutil.TempDir(t)
		defer clearDir()

		notifier := &fakeNotifier{}

		nc := notification.NewWithCustomLanguageFetcher(translator.New(catalog.NewBuilder()), DefaultNotificationPolicy{}, func() (language.Tag, error) {
			return language.English, nil
		}, map[string]notification.Notifier{"fake": notifier})

		detector := &fakeDetector{t: t}

		noAdditionalActions := func([]core.Detector, dbconn.RwConn) error { return nil }

		Convey("Test Insights Generation", func() {
			e, err := NewCustomEngine(dir, nc, core.Options{}, func(c *creator, o core.Options) []core.Detector {
				detector.creator = c
				return []core.Detector{detector}
			}, noAdditionalActions)

			So(err, ShouldBeNil)

			defer func() {
				So(e.Close(), ShouldBeNil)
			}()

			doneWithRun := make(chan struct{})

			go func() {
				runDatabaseWriterLoop(e)
				doneWithRun <- struct{}{}
			}()

			clock := &insighttestsutil.FakeClock{Time: testutil.MustParseTime(`2000-01-01 00:00:00 +0000`)}

			step := func(v *fakeValue) {
				if v != nil {
					detector.setValue(v)
				}

				execOnDetectors(e.txActions, e.core.Detectors, clock)
				time.Sleep(time.Millisecond * 100)
				clock.Sleep(time.Second * 1)
			}

			genInsight := func(v fakeValue) {
				step(&v)
			}

			nopStep := func() {
				step(nil)
			}

			nopStep()
			nopStep()
			genInsight(fakeValue{Category: core.LocalCategory, Content: content{T: "42"}, Rating: core.BadRating})
			nopStep()
			nopStep()
			genInsight(fakeValue{Category: core.IntelCategory, Content: content{T: "35"}, Rating: core.OkRating})
			nopStep()
			genInsight(fakeValue{Category: core.ComparativeCategory, Content: content{T: "13"}, Rating: core.BadRating})

			// stop main loop
			close(e.txActions)

			_, ok := <-doneWithRun

			So(ok, ShouldBeTrue)

			// Notify only bad-rating insights
			So(len(notifier.notifications), ShouldEqual, 2)

			{
				n, ok := notifier.notifications[0].Content.(core.InsightProperties)
				So(ok, ShouldBeTrue)
				So(notifier.notifications[0].ID, ShouldEqual, 1)
				So(n.Content, ShouldResemble, content{T: "42"})
			}

			{
				n, ok := notifier.notifications[1].Content.(core.InsightProperties)
				So(ok, ShouldBeTrue)
				So(notifier.notifications[1].ID, ShouldEqual, 3)
				So(n.Content, ShouldResemble, content{T: "13"})
			}

			fetcher := e.Fetcher()

			Convey("fetch all insights with no filter, sorting by time, default (desc) order", func() {
				insights, err := fetcher.FetchInsights(dummyContext, core.FetchOptions{
					Interval: data.TimeInterval{
						From: testutil.MustParseTime(`2000-01-01 00:00:00 +0000`),
						To:   testutil.MustParseTime(`2000-01-01 22:00:00 +0000`),
					},
				})

				So(err, ShouldBeNil)

				So(len(insights), ShouldEqual, 3)

				So(insights[0].Category(), ShouldEqual, core.ComparativeCategory)
				So(insights[0].Content().(*content).T, ShouldEqual, "13")
				So(insights[0].ID(), ShouldEqual, 3)
				So(insights[0].Rating(), ShouldEqual, core.BadRating)
				So(insights[0].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:07 +0000`))

				So(insights[1].Category(), ShouldEqual, core.IntelCategory)
				So(insights[1].Content().(*content).T, ShouldEqual, "35")
				So(insights[1].ID(), ShouldEqual, 2)
				So(insights[1].Rating(), ShouldEqual, core.OkRating)
				So(insights[1].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:05 +0000`))

				So(insights[2].Category(), ShouldEqual, core.LocalCategory)
				So(insights[2].Content().(*content).T, ShouldEqual, "42")
				So(insights[2].ID(), ShouldEqual, 1)
				So(insights[2].Rating(), ShouldEqual, core.BadRating)
				So(insights[2].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:02 +0000`))
			})

			Convey("fetch 2 most recent insights", func() {
				insights, err := fetcher.FetchInsights(dummyContext, core.FetchOptions{
					Interval: data.TimeInterval{
						From: testutil.MustParseTime(`2000-01-01 00:00:00 +0000`),
						To:   testutil.MustParseTime(`2000-01-01 22:00:00 +0000`),
					},
					MaxEntries: 2,
				})

				So(err, ShouldBeNil)

				So(len(insights), ShouldEqual, 2)

				So(insights[0].Category(), ShouldEqual, core.ComparativeCategory)
				So(insights[0].Content().(*content).T, ShouldEqual, "13")
				So(insights[0].ID(), ShouldEqual, 3)
				So(insights[0].Rating(), ShouldEqual, core.BadRating)
				So(insights[0].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:07 +0000`))

				So(insights[1].Category(), ShouldEqual, core.IntelCategory)
				So(insights[1].Content().(*content).T, ShouldEqual, "35")
				So(insights[1].ID(), ShouldEqual, 2)
				So(insights[1].Rating(), ShouldEqual, core.OkRating)
				So(insights[1].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:05 +0000`))
			})

			Convey("fetch all insights with no filter, sorting by time, asc order", func() {
				insights, err := fetcher.FetchInsights(dummyContext, core.FetchOptions{
					Interval: data.TimeInterval{
						From: testutil.MustParseTime(`2000-01-01 00:00:00 +0000`),
						To:   testutil.MustParseTime(`2000-01-01 22:00:00 +0000`),
					},
					OrderBy: core.OrderByCreationAsc,
				})

				So(err, ShouldBeNil)

				So(len(insights), ShouldEqual, 3)

				So(insights[0].Category(), ShouldEqual, core.LocalCategory)
				So(insights[0].Content().(*content).T, ShouldEqual, "42")
				So(insights[0].ID(), ShouldEqual, 1)
				So(insights[0].Rating(), ShouldEqual, core.BadRating)
				So(insights[0].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:02 +0000`))

				So(insights[1].Category(), ShouldEqual, core.IntelCategory)
				So(insights[1].Content().(*content).T, ShouldEqual, "35")
				So(insights[1].ID(), ShouldEqual, 2)
				So(insights[1].Rating(), ShouldEqual, core.OkRating)
				So(insights[1].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:05 +0000`))

				So(insights[2].Category(), ShouldEqual, core.ComparativeCategory)
				So(insights[2].Content().(*content).T, ShouldEqual, "13")
				So(insights[2].ID(), ShouldEqual, 3)
				So(insights[2].Rating(), ShouldEqual, core.BadRating)
				So(insights[2].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:07 +0000`))
			})

			Convey("fetch intel category, asc order", func() {
				insights, err := fetcher.FetchInsights(dummyContext, core.FetchOptions{
					Interval: data.TimeInterval{
						From: testutil.MustParseTime(`2000-01-01 00:00:00 +0000`),
						To:   testutil.MustParseTime(`2000-01-01 22:00:00 +0000`),
					},
					OrderBy:  core.OrderByCreationAsc,
					FilterBy: core.FilterByCategory,
					Category: core.IntelCategory,
				})

				So(err, ShouldBeNil)

				So(len(insights), ShouldEqual, 1)

				So(insights[0].Category(), ShouldEqual, core.IntelCategory)
				So(insights[0].Content().(*content).T, ShouldEqual, "35")
				So(insights[0].ID(), ShouldEqual, 2)
				So(insights[0].Rating(), ShouldEqual, core.OkRating)
				So(insights[0].Time(), ShouldEqual, testutil.MustParseTime(`2000-01-01 00:00:05 +0000`))
			})
		})

		Convey("Test Insights Samples generated when the application starts", func() {
			e, err := NewCustomEngine(dir, nc, core.Options{}, func(c *creator, o core.Options) []core.Detector {
				detector.creator = c
				return []core.Detector{detector}
			},
				addInsightsSamples,
			)

			So(err, ShouldBeNil)

			fetcher := e.Fetcher()

			sampleInsights, err := fetcher.FetchInsights(dummyContext, core.FetchOptions{
				Interval: data.TimeInterval{
					From: testutil.MustParseTime("0000-01-01 00:00:00 +0000"),
					To:   testutil.MustParseTime("4000-01-01 00:00:00 +0000"),
				},
			})

			So(err, ShouldBeNil)

			So(len(sampleInsights), ShouldEqual, 1)
		})

		Convey("Test engine loop", func() {
			e, err := NewCustomEngine(dir, nc, core.Options{}, func(c *creator, o core.Options) []core.Detector {
				detector.creator = c
				return []core.Detector{detector}
			},
				noAdditionalActions,
			)

			So(err, ShouldBeNil)

			defer func() {
				So(e.Close(), ShouldBeNil)
			}()

			// Generate one insight, on the first cycle
			detector.setValue(&fakeValue{Category: core.LocalCategory, Content: content{D: "content"}, Rating: core.BadRating})

			done, cancel := e.Run()

			time.Sleep(time.Second * 3)

			cancel()
			done()

			So(len(notifier.notifications), ShouldEqual, 1)

			n, ok := notifier.notifications[0].Content.(core.InsightProperties)
			So(ok, ShouldBeTrue)
			So(n.Content, ShouldResemble, content{D: "content"})
		})
	})
}
