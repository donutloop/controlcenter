package logeater

import (
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/lightmeter/controlcenter/data"
	"strings"
	"testing"
	"time"
)

type FakePublisher struct {
	logs []data.Record
}

func (this *FakePublisher) Publish(r data.Record) {
	this.logs = append(this.logs, r)
}

func (FakePublisher) Close() {
}

func TestReadingLogs(t *testing.T) {
	Convey("Read From Reader", t, func() {
		pub := FakePublisher{}

		Convey("Read Nothing", func() {
			reader := strings.NewReader(``)
			ReadFromReader(reader, &pub)
			So(len(pub.logs), ShouldEqual, 0)
		})

		Convey("Ignore Wrong Line", func() {
			reader := strings.NewReader(`Not a valid log line`)
			ReadFromReader(reader, &pub)
			So(len(pub.logs), ShouldEqual, 0)
		})

		Convey("Accepts line with error on reading the payload (but header is okay)", func() {
			reader := strings.NewReader(`Mar  1 07:42:10 mail opendkim[225]: C11EA2C620C7: not authenticated`)
			ReadFromReader(reader, &pub)
			So(len(pub.logs), ShouldEqual, 1)
			So(pub.logs[0].Payload, ShouldEqual, nil)
			So(pub.logs[0].Header.Time.Day, ShouldEqual, 1)
			So(pub.logs[0].Header.Time.Month, ShouldEqual, time.March)
			So(pub.logs[0].Header.Time.Hour, ShouldEqual, 7)
			So(pub.logs[0].Header.Time.Minute, ShouldEqual, 42)
			So(pub.logs[0].Header.Time.Second, ShouldEqual, 10)
		})

		Convey("Read three lines, one of them with invalid payload", func() {
			reader := strings.NewReader(`

Sep 16 00:07:43 smtpnode07 postfix-10.20.30.40/smtp[3022]: 0C31D3D1E6: to=<a@b.c>, relay=a.net[1.2.3.4]:25, delay=1, delays=0/0.9/0.69/0.03, dsn=4.7.0, status=deferred Extra text)
Nov  1 07:42:10 mail opendkim[225]: C11EA2C620C7: not authenticated

Dec 16 14:08:45 smtpnode07 postfix-10.20.30.40/smtp[3022]: 0C31D3D1E6: to=<a@b.c>, relay=a.net[1.2.3.4]:25, delay=1, delays=0/0.9/0.69/0.03, dsn=4.7.0, status=deferred Extra text)
			`)
			ReadFromReader(reader, &pub)
			So(len(pub.logs), ShouldEqual, 3)
			So(pub.logs[0].Payload, ShouldNotEqual, nil)
			So(pub.logs[1].Payload, ShouldEqual, nil)
			So(pub.logs[2].Payload, ShouldNotEqual, nil)
		})

	})
}