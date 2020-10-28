package localrblinsight

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gitlab.com/lightmeter/controlcenter/data"
	"gitlab.com/lightmeter/controlcenter/i18n/translator"
	"gitlab.com/lightmeter/controlcenter/insights/core"
	"gitlab.com/lightmeter/controlcenter/localrbl"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"log"
	"net"
	"time"
)

type Options struct {
	Checker                     localrbl.Checker
	CheckInterval               time.Duration
	RetryOnScanErrorInterval    time.Duration
	MinTimeToGenerateNewInsight time.Duration
}

type content struct {
	ScanInterval data.TimeInterval         `json:"scan_interval"`
	Address      net.IP                    `json:"address"`
	RBLs         []localrbl.ContentElement `json:"rbls"`
}

func (c content) String() string {
	return translator.Stringfy(c)
}

func (c content) TplString() string {
	return translator.I18n("The IP address %%v is listed by %%v RBLs")
}

func (c content) Args() []interface{} {
	return []interface{}{c.Address, len(c.RBLs)}
}

const (
	ContentType   = "local_rbl_check"
	ContentTypeId = 4
)

func decodeContent(b []byte) (*content, error) {
	content := content{}
	err := json.Unmarshal(b, &content)

	if err != nil {
		return nil, errorutil.Wrap(err)
	}

	return &content, nil
}

func init() {
	core.RegisterContentType(ContentType, ContentTypeId, func(b []byte) (core.Content, error) {
		return decodeContent(b)
	})
}

type detector struct {
	options Options
	creator core.Creator
}

func (d *detector) Close() error {
	return d.options.Checker.Close()
}

func getDetectorOptions(options core.Options) Options {
	detectorOptions, ok := options["localrbl"].(Options)

	if !ok {
		errorutil.MustSucceed(errors.New("Invalid detector options"))
	}

	return detectorOptions
}

func NewDetector(creator core.Creator, options core.Options) core.Detector {
	detectorOptions := getDetectorOptions(options)

	return &detector{
		options: detectorOptions,
		creator: creator,
	}
}

func shouldGenerateBasedOnHistoricalDataAndCurrentResults(ctx context.Context, d *detector, r localrbl.Results, c core.Clock, tx *sql.Tx) (bool, error) {
	now := c.Now()

	lookbackTime := now.Add(-d.options.MinTimeToGenerateNewInsight)

	var lastInsightRawContent string

	err := tx.QueryRowContext(ctx, `select
			content
		from
			insights
		where
			time >= ? and content_type = ?
		order by
			time desc
		limit
			1
		`, lookbackTime.Unix(), ContentTypeId).Scan(&lastInsightRawContent)

	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}

	if err != nil {
		return false, errorutil.Wrap(err)
	}

	lastInsightContent, err := decodeContent([]byte(lastInsightRawContent))

	if err != nil {
		return false, errorutil.Wrap(err)
	}

	resultChanged := contentsHaveDifferentLists(r.RBLs, lastInsightContent.RBLs)

	if !resultChanged {
		log.Println("RBL Scan result will not generate a new insight as scan results has not changed since last insight")
	}

	return resultChanged, nil
}

// Given two RBL lists, were they generated by different RBLs?
// It assumes the lists are already sorted
func contentsHaveDifferentLists(a, b []localrbl.ContentElement) bool {
	if len(a) != len(b) {
		return true
	}

	for i, v := range a {
		if v != b[i] {
			return true
		}
	}

	return false
}

func maybeCreateInsightForResult(ctx context.Context, d *detector, r localrbl.Results, c core.Clock, tx *sql.Tx) error {
	shouldGenerate, err := shouldGenerateBasedOnHistoricalDataAndCurrentResults(ctx, d, r, c, tx)

	if err != nil {
		return errorutil.Wrap(err)
	}

	if !shouldGenerate {
		return nil
	}

	return generateInsight(tx, c, d.creator, content{
		ScanInterval: r.Interval,
		Address:      d.options.Checker.IPAddress(context.Background()),
		RBLs:         r.RBLs,
	})
}

const (
	detectionKind = "local_rbl_scan_start"
)

func maybeStartANewScan(d *detector, c core.Clock, tx *sql.Tx) error {
	now := c.Now()

	// If it's time, ask the checker to start a new scan
	t, err := core.RetrieveLastDetectorExecution(tx, detectionKind)

	if err != nil {
		return errorutil.Wrap(err)
	}

	if !t.IsZero() && !now.After(t.Add(d.options.CheckInterval)) {
		return nil
	}

	d.options.Checker.NotifyNewScan(now)

	if err := core.StoreLastDetectorExecution(tx, detectionKind, now); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func scheduleANewScanShortly(d *detector, c core.Clock, tx *sql.Tx) error {
	lastExecutedTime := c.Now().Add(d.options.RetryOnScanErrorInterval).Add(-d.options.CheckInterval)

	if err := core.StoreLastDetectorExecution(tx, detectionKind, lastExecutedTime); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func (d *detector) Step(c core.Clock, tx *sql.Tx) error {
	baseCtx := context.Background()

	return d.options.Checker.Step(c.Now(), func(r localrbl.Results) error {
		ctx, cancel := context.WithTimeout(baseCtx, time.Second*2)

		defer cancel()

		if r.Err == nil {
			// a scan result is available
			return maybeCreateInsightForResult(ctx, d, r, c, tx)
		}

		// A scan just ended with an error, schedule a new scan shortly after the current failure
		return scheduleANewScanShortly(d, c, tx)
	}, func() error {
		// no scan result available
		return maybeStartANewScan(d, c, tx)
	})
}

func generateInsight(tx *sql.Tx, c core.Clock, creator core.Creator, content content) error {
	properties := core.InsightProperties{
		Time:        c.Now(),
		Category:    core.LocalCategory,
		Rating:      core.BadRating,
		ContentType: ContentType,
		Content:     content,
	}

	if err := creator.GenerateInsight(tx, properties); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

// Executed only on development builds, for better developer experience
func (d *detector) GenerateSampleInsight(tx *sql.Tx, c core.Clock) error {
	if err := generateInsight(tx, c, d.creator, content{
		ScanInterval: data.TimeInterval{From: c.Now(), To: c.Now().Add(time.Second * 30)},
		Address:      d.options.Checker.IPAddress(context.Background()),
		RBLs: []localrbl.ContentElement{
			{RBL: "rbl.com", Text: "Funny reason"},
			{RBL: "anotherrbl.de", Text: "Another funny reason"},
		},
	}); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}
