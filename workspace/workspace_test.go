package workspace

import (
	. "github.com/smartystreets/goconvey/convey"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3"
	"gitlab.com/lightmeter/controlcenter/logdb"
	"gitlab.com/lightmeter/controlcenter/util/testutil"
	"os"
	"testing"
	"time"
)

func init() {
	lmsqlite3.Initialize(lmsqlite3.Options{})
}


func TestWorkspaceCreation(t *testing.T) {
	Convey("Creation fails on several scenarios", t, func() {
		Convey("No Permission on workspace", func() {
			// FIXME: this is relying on linux properties, as /proc is a read-only directory
			_, err := NewWorkspace("/proc/lalala", logdb.Config{Location: time.UTC})
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Creation succeeds", t, func() {
		Convey("Create Workspace", func() {
			dir := testutil.TempDir()
			defer os.RemoveAll(dir)
			ws, err := NewWorkspace(dir, logdb.Config{Location: time.UTC})
			So(err, ShouldBeNil)

			defer ws.Close()
			So(ws.HasLogs(), ShouldBeFalse)
		})

		Convey("Empty Database is properly closed", func() {
			dir := testutil.TempDir()
			defer os.RemoveAll(dir)
			ws, err := NewWorkspace(dir, logdb.Config{Location: time.UTC})
			So(err, ShouldBeNil)
			So(ws.HasLogs(), ShouldBeFalse)
			So(ws.Close(), ShouldBeNil)
		})

		Convey("Reopening workspace succeeds", func() {
			dir := testutil.TempDir()
			defer os.RemoveAll(dir)

			ws1, err := NewWorkspace(dir, logdb.Config{Location: time.UTC})
			ws1.Close()

			ws2, err := NewWorkspace(dir, logdb.Config{Location: time.UTC})
			So(err, ShouldBeNil)
			ws2.Close()
		})
	})
}
