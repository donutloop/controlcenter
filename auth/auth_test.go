package auth

import (
	"errors"
	. "github.com/smartystreets/goconvey/convey"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func tempDir() string {
	dir, e := ioutil.TempDir("", "lightmeter-tests-*")
	if e != nil {
		panic("error creating temp dir")
	}
	return dir
}

func TestSessionKey(t *testing.T) {
	Convey("Test Session Key", t, func() {
		dir := tempDir()
		defer os.RemoveAll(dir)
		var generatedKey, recoveredKey [][]byte

		// NOTE: for now we are generating only one key, but
		// gennerating multiple ones is desirable
		{
			auth, _ := NewAuth(path.Join(dir))
			generatedKey = auth.SessionKeys()
			So(generatedKey, ShouldNotBeNil)
			So(len(generatedKey), ShouldEqual, 1)
			So(generatedKey[0], ShouldNotBeNil)
		}

		{
			auth, _ := NewAuth(path.Join(dir))
			recoveredKey = auth.SessionKeys()
		}

		So(recoveredKey, ShouldResemble, generatedKey)
	})
}

func TestAuth(t *testing.T) {
	strongPassword := `ghjzfpailduifiapdq9um6ysuubvtjywAqbnadq+aUerxrqhfp`

	Convey("Test Auth", t, func() {
		dir := tempDir()
		defer os.RemoveAll(dir)
		auth, err := NewAuth(path.Join(dir))
		So(err, ShouldBeNil)
		So(auth, ShouldNotBeNil)

		Convey("No user is initially registred", func() {
			ok, err := auth.HasAnyUser()
			So(err, ShouldBeNil)
			So(ok, ShouldBeFalse)

			Convey("Login fails", func() {
				ok, _, err := auth.Authenticate("user@example.com", "password")
				So(ok, ShouldBeFalse)
				So(err, ShouldBeNil)
			})
		})

		Convey("Register User Fails", func() {
			Convey("Empty password", func() {
				err := auth.Register("user@example.com", "Name Surname", "")
				So(errors.Is(err, ErrWeakPassword), ShouldBeTrue)
			})

			Convey("Password is equal to email", func() {
				err := auth.Register("user@example.com", "Name Surname", "user@example.com")
				So(errors.Is(err, ErrWeakPassword), ShouldBeTrue)
			})

			Convey("Password is equal to name", func() {
				err := auth.Register("user@example.com", strongPassword, strongPassword)
				So(errors.Is(err, ErrWeakPassword), ShouldBeTrue)
			})

			Convey("Invalid email", func() {
				err := auth.Register("not@an@email.com", "Name Surname", strongPassword)
				So(errors.Is(err, ErrInvalidEmail), ShouldBeTrue)
			})

			Convey("Dictionary password", func() {
				err := auth.Register("user@email.com", "Name Surname", "ElvisForever")
				So(errors.Is(err, ErrWeakPassword), ShouldBeTrue)
			})

			Convey("Empty Name", func() {
				err := auth.Register("user@email.com", "   ", strongPassword)
				So(errors.Is(err, ErrInvalidName), ShouldBeTrue)
			})
		})

		Convey("Register Multiple Users", func() {
			user1Passwd := `ymzlxzmojdnQ3revu/s2jnqbFydoqw`
			user2Passwd := `yp9nr1yog|cWzjDftgspdgkntkbjig`

			So(auth.Register("user.one@example.com", "User One", user1Passwd), ShouldBeNil)
			So(auth.Register("user.two@example.com", "User Two", user2Passwd), ShouldBeNil)

			Convey("Passwords do not mix", func() {
				{
					ok, _, err := auth.Authenticate("user.one@example.com", user2Passwd)
					So(err, ShouldBeNil)
					So(ok, ShouldBeFalse)
				}

				{
					ok, _, err := auth.Authenticate("user.two@example.com", user1Passwd)
					So(err, ShouldBeNil)
					So(ok, ShouldBeFalse)
				}

				{
					ok, _, err := auth.Authenticate("user.two@example.com", user2Passwd)
					So(err, ShouldBeNil)
					So(ok, ShouldBeTrue)
				}
			})
		})

		Convey("Register User", func() {
			err := auth.Register("user@example.com", "Name Surname", strongPassword)
			So(err, ShouldBeNil)
			ok, err := auth.HasAnyUser()
			So(err, ShouldBeNil)
			So(ok, ShouldBeTrue)

			Convey("Registering the same user again fails", func() {
				err := auth.Register("user@example.com", "Another Surname", strongPassword)
				So(err, ShouldNotBeNil)
			})

			Convey("Login fails with wrong user", func() {
				ok, _, err := auth.Authenticate("wrong_user@example.com", strongPassword)
				So(ok, ShouldBeFalse)
				So(err, ShouldBeNil)
			})

			Convey("Login fails with wrong password", func() {
				ok, _, err := auth.Authenticate("user@example.com", "654321")
				So(ok, ShouldBeFalse)
				So(err, ShouldBeNil)
			})

			Convey("Login succeeds", func() {
				ok, userData, err := auth.Authenticate("user@example.com", strongPassword)
				So(ok, ShouldBeTrue)
				So(err, ShouldBeNil)
				So(userData.Id, ShouldEqual, 1)
				So(userData.Email, ShouldEqual, "user@example.com")
				So(userData.Name, ShouldEqual, "Name Surname")
			})
		})
	})
}