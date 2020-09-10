package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"
	mock_dashboard "gitlab.com/lightmeter/controlcenter/dashboard/mock"
	"gitlab.com/lightmeter/controlcenter/data"
	parser "gitlab.com/lightmeter/postfix-log-parser"
)

func TestDashboard(t *testing.T) {
	ctrl := gomock.NewController(t)

	m := mock_dashboard.NewMockDashboard(ctrl)

	Convey("CountByStatus", t, func() {
		Convey("No Time Interval", func() {
			s := httptest.NewServer(countByStatusHandler{dashboard: m, timezone: time.UTC})
			r, err := http.Get(s.URL)
			So(err, ShouldBeNil)
			So(r.StatusCode, ShouldEqual, http.StatusUnprocessableEntity)
		})

		Convey("Dates out of order", func() {
			s := httptest.NewServer(countByStatusHandler{dashboard: m, timezone: time.UTC})
			// "from" comes after "to"
			r, err := http.Get(fmt.Sprintf("%s?to=1999-01-01&from=1999-12-31", s.URL))
			So(err, ShouldBeNil)
			So(r.StatusCode, ShouldEqual, http.StatusUnprocessableEntity)
		})

		Convey("Success", func() {
			interval, err := data.ParseTimeInterval("1999-01-01", "1999-12-31", time.UTC)
			So(err, ShouldBeNil)

			m.EXPECT().CountByStatus(parser.SentStatus, interval).Return(4)
			m.EXPECT().CountByStatus(parser.DeferredStatus, interval).Return(3)
			m.EXPECT().CountByStatus(parser.BouncedStatus, interval).Return(2)

			s := httptest.NewServer(countByStatusHandler{dashboard: m, timezone: time.UTC})
			// "from" comes after "to"
			r, err := http.Get(fmt.Sprintf("%s?from=1999-01-01&to=1999-12-31", s.URL))
			ctrl.Finish()
			So(err, ShouldBeNil)
			So(r.StatusCode, ShouldEqual, http.StatusOK)

			var body map[string]interface{}
			dec := json.NewDecoder(r.Body)
			err = dec.Decode(&body)
			So(err, ShouldBeNil)

			// NOTE: all numbers are decoded into an interface{} as float64, so we want to have float64 here, too.
			expected := map[string]interface{}{"bounced": float64(2), "deferred": float64(3), "sent": float64(4)}
			So(body, ShouldResemble, expected)
		})
	})
}