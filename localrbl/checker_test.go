package localrbl

import (
	"github.com/mrichman/godnsbl"
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/dbconn"
	"gitlab.com/lightmeter/controlcenter/meta"
	"gitlab.com/lightmeter/controlcenter/util/testutil"
	"net"
	"path"
	"strings"
	"testing"
	"time"
)

func init() {
	lmsqlite3.Initialize(lmsqlite3.Options{})
}

func TestDnsRBL(t *testing.T) {
	Convey("Test Local RBL", t, func() {
		dir, clearDir := testutil.TempDir()
		defer clearDir()

		connPair, err := dbconn.NewConnPair(path.Join(dir, "master.db"))
		So(err, ShouldBeNil)

		meta, err := meta.NewMetaDataHandler(connPair, "master")
		So(err, ShouldBeNil)

		lookup := func(rblList string, targetHost string) godnsbl.RBLResults {
			// the sleep here is just to "simulate" an actual call,
			// that is not instantaneous
			time.Sleep(200 * time.Millisecond)

			if !strings.HasSuffix(rblList, "-blocked") {
				return godnsbl.RBLResults{}
			}

			return godnsbl.RBLResults{
				Host:    targetHost,
				List:    rblList,
				Results: []godnsbl.Result{{Listed: true, Address: targetHost, Text: "Some Error", Rbl: rblList}},
			}
		}

		Convey("An IP address is defined", func() {
			{
				settings := Settings{
					LocalIP: net.ParseIP("11.22.33.44"),
				}

				meta.StoreJson(SettingsKey, &settings)
			}

			Convey("Panic on invalid number of workers", func() {
				So(func() {
					newDnsChecker(meta, Options{
						Lookup:           lookup,
						NumberOfWorkers:  0, // cannot have less than 1 worker!
						RBLProvidersURLs: []string{"rbl1", "rbl2", "rbl3", "rbl4", "rbl5"},
					})
				}, ShouldPanic)
			})

			Convey("Panic if lookup function is not defined", func() {
				So(func() {
					newDnsChecker(meta, Options{
						NumberOfWorkers:  2,
						RBLProvidersURLs: []string{"rbl1", "rbl2", "rbl3", "rbl4", "rbl5"},
					})
				}, ShouldPanic)
			})

			Convey("Not blocked in any lists", func() {
				checker := newDnsChecker(meta, Options{
					Lookup:           lookup,
					NumberOfWorkers:  2,
					RBLProvidersURLs: []string{"rbl1", "rbl2", "rbl3", "rbl4", "rbl5"},
				})

				defer checker.Close()

				checker.StartListening()

				baseTime := testutil.MustParseTime(`2000-01-01 00:00:00 +0000`)

				checker.NotifyNewScan(baseTime)

				time.Sleep(700 * time.Millisecond)

				select {
				case <-checker.checkerResultsChan:
					So(false, ShouldBeTrue)
				default:
				}
			})

			Convey("Blocked in some RBLs", func() {
				checker := newDnsChecker(meta, Options{
					Lookup:           lookup,
					NumberOfWorkers:  2,
					RBLProvidersURLs: []string{"rbl1-blocked", "rbl2", "rbl3-blocked", "rbl4-blocked", "rbl5"},
				})

				defer checker.Close()

				checker.StartListening()

				baseTime := testutil.MustParseTime(`2000-01-01 00:00:00 +0000`)

				checker.NotifyNewScan(baseTime)

				time.Sleep(700 * time.Millisecond)

				select {
				case r := <-checker.checkerResultsChan:
					So(r.RBLs, ShouldResemble, []ContentElement{
						{RBL: "rbl1-blocked", Text: "Some Error"},
						{RBL: "rbl3-blocked", Text: "Some Error"},
						{RBL: "rbl4-blocked", Text: "Some Error"},
					})

					So(r.Interval.From, ShouldResemble, baseTime)
					So(r.Interval.To.After(r.Interval.From), ShouldBeTrue)
				default:
					So(false, ShouldBeTrue)
				}
			})
		})

		Convey("Do not scan if IP address is not defined", func() {
			checker := newDnsChecker(meta, Options{
				Lookup:           lookup,
				NumberOfWorkers:  2,
				RBLProvidersURLs: []string{"rbl1-blocked", "rbl2", "rbl3-blocked", "rbl4-blocked", "rbl5"},
			})

			defer checker.Close()

			checker.StartListening()

			baseTime := testutil.MustParseTime(`2000-01-01 00:00:00 +0000`)

			checker.NotifyNewScan(baseTime)

			time.Sleep(700 * time.Millisecond)

			select {
			case <-checker.checkerResultsChan:
				So(false, ShouldBeTrue)
			default:
			}
		})
	})
}