package parser

import (
	"errors"
	"net"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestErrorKinds(t *testing.T) {
	Convey("Some errors still allows to recover header info", t, func() {
		So(IsRecoverableError(nil), ShouldBeTrue)
		So(IsRecoverableError(ErrUnsupportedLogLine), ShouldBeTrue)
		So(IsRecoverableError(ErrInvalidHeaderLine), ShouldBeFalse)
		So(IsRecoverableError(errors.New("Some Random Error")), ShouldBeFalse)
	})
}

func TestParsingInvalidLines(t *testing.T) {
	Convey("Invalid Line", t, func() {
		_, p, err := Parse([]byte("Invalid Line"))
		So(p, ShouldBeNil)
		So(err, ShouldEqual, ErrInvalidHeaderLine)
	})
}

func TestParsingUnsupportedGeneralMessage(t *testing.T) {
	Convey("Unsupported Smtp Line", t, func() {
		h, p, err := Parse([]byte(`Sep 16 00:07:41 smtp-node07.com postfix-10.20.30.40/smtp[31868]: 0D59F4165A:` +
			` host mx-aol.mail.gm0.yahoodns.net[44.55.66.77] said: 421 4.7.0 [TSS04] ` +
			`Messages from 10.20.30.40 temporarily deferred due to user complaints - 4.16.55.1;i ` +
			`see https://help.yahoo.com/kb/postmaster/SLN3434.html (in reply to MAIL FROM command)`))

		So(p, ShouldBeNil)
		So(err, ShouldEqual, ErrUnsupportedLogLine)
		So(h.Process, ShouldEqual, "postfix")
		So(h.Daemon, ShouldEqual, "smtp")
		So(h.Time.Day, ShouldEqual, 16)
		So(h.Time.Month.String(), ShouldEqual, "September")
		So(h.Time.Hour, ShouldEqual, 0)
		So(h.Time.Minute, ShouldEqual, 7)
		So(h.Time.Second, ShouldEqual, 41)
		So(h.Host, ShouldEqual, "smtp-node07.com")
	})

	Convey("Unsupported Log Line", t, func() {
		_, p, err := Parse([]byte(`Feb 16 00:07:34 smtpnode07 postfix-10.20.30.40/something[2342]: unsupported message`))
		So(p, ShouldBeNil)
		So(err, ShouldEqual, ErrUnsupportedLogLine)
	})

	Convey("Line with leading null chars should remove them and parse properly", t, func() {
		// many leading zero bytes, and then "Jun  6 23:34:18 ucs fetchmail[2174]: Nachricht h-796080212e8fae@h-36a01c19fbb.com@h-615c50f91311c26bed347e:39 von 41 wird gelesen (3157 Bytes im Nachrichtenkopf) (Log-Meldung unvollständig)"
		line := []byte{
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x4a, 0x75, 0x6e, 0x20, 0x20, 0x36,
			0x20, 0x32, 0x33, 0x3a, 0x33, 0x34, 0x3a, 0x31, 0x38, 0x20, 0x75, 0x63,
			0x73, 0x20, 0x66, 0x65, 0x74, 0x63, 0x68, 0x6d, 0x61, 0x69, 0x6c, 0x5b,
			0x32, 0x31, 0x37, 0x34, 0x5d, 0x3a, 0x20, 0x4e, 0x61, 0x63, 0x68, 0x72,
			0x69, 0x63, 0x68, 0x74, 0x20, 0x68, 0x2d, 0x37, 0x39, 0x36, 0x30, 0x38,
			0x30, 0x32, 0x31, 0x32, 0x65, 0x38, 0x66, 0x61, 0x65, 0x40, 0x68, 0x2d,
			0x33, 0x36, 0x61, 0x30, 0x31, 0x63, 0x31, 0x39, 0x66, 0x62, 0x62, 0x2e,
			0x63, 0x6f, 0x6d, 0x40, 0x68, 0x2d, 0x36, 0x31, 0x35, 0x63, 0x35, 0x30,
			0x66, 0x39, 0x31, 0x33, 0x31, 0x31, 0x63, 0x32, 0x36, 0x62, 0x65, 0x64,
			0x33, 0x34, 0x37, 0x65, 0x3a, 0x33, 0x39, 0x20, 0x76, 0x6f, 0x6e, 0x20,
			0x34, 0x31, 0x20, 0x77, 0x69, 0x72, 0x64, 0x20, 0x67, 0x65, 0x6c, 0x65,
			0x73, 0x65, 0x6e, 0x20, 0x28, 0x33, 0x31, 0x35, 0x37, 0x20, 0x42, 0x79,
			0x74, 0x65, 0x73, 0x20, 0x69, 0x6d, 0x20, 0x4e, 0x61, 0x63, 0x68, 0x72,
			0x69, 0x63, 0x68, 0x74, 0x65, 0x6e, 0x6b, 0x6f, 0x70, 0x66, 0x29, 0x20,
			0x28, 0x4c, 0x6f, 0x67, 0x2d, 0x4d, 0x65, 0x6c, 0x64, 0x75, 0x6e, 0x67,
			0x20, 0x75, 0x6e, 0x76, 0x6f, 0x6c, 0x6c, 0x73, 0x74, 0xc3, 0xa4, 0x6e,
			0x64, 0x69, 0x67, 0x29,
		}

		_, p, err := Parse(line)
		So(p, ShouldBeNil)
		So(err, ShouldEqual, ErrUnsupportedLogLine)
	})

	Convey("Unsupported Log Line with slash on process", t, func() {
		h, p, err := Parse([]byte(`Mar  3 02:55:42 mail process/daemon/with/slash[9708]: any content here`))
		So(err, ShouldEqual, ErrUnsupportedLogLine)
		So(p, ShouldBeNil)
		So(h.Process, ShouldEqual, "process")
		So(h.Daemon, ShouldEqual, "daemon/with/slash")
		So(h.PID, ShouldEqual, 9708)
	})

	Convey("Unsupported opendkim line, but time is okay", t, func() {
		h, p, err := Parse([]byte(`Apr  5 19:00:02 mail opendkim[195]: 407032C4FF6A: DKIM-Signature field added (s=mail, d=lightmeter.io)`))
		So(err, ShouldEqual, ErrUnsupportedLogLine)
		So(p, ShouldBeNil)
		So(h.Process, ShouldEqual, "opendkim")
		So(h.PID, ShouldEqual, 195)
		So(h.Time.Month, ShouldEqual, time.April)
		So(h.Time.Day, ShouldEqual, 5)
		So(h.Time.Hour, ShouldEqual, 19)
		So(h.Time.Minute, ShouldEqual, 0)
		So(h.Time.Second, ShouldEqual, 2)
	})

	Convey("Unsupported dovecot line", t, func() {
		h, p, err := Parse([]byte(`May  5 18:56:52 mail dovecot: imap(laal@mail.io)<28358><CO3htpid9tRXDNo5>: Connection closed (IDLE running for 0.001 + waiting input for 28.914 secs, 2 B in + 10 B out, state=wait-input) in=703 out=12338 deleted=0 expunged=0 trashed=0 hdr_count=0 hdr_bytes=0 body_count=0 body_bytes=0`))
		So(err, ShouldEqual, ErrUnsupportedLogLine)
		So(p, ShouldBeNil)
		So(h.Process, ShouldEqual, "dovecot")
		So(h.PID, ShouldEqual, 0)
	})

	Convey("Unsupported line starting with slash", t, func() {
		h, p, err := Parse([]byte(`Dec 17 06:25:48 sm02 /postfix-script[112854]: the Postfix mail system is running: PID: 95072`))
		So(err, ShouldEqual, ErrUnsupportedLogLine)
		So(p, ShouldBeNil)
		So(h.Process, ShouldEqual, "postfix-script")
		So(h.PID, ShouldEqual, 112854)
	})

}

func TestSMTPParsing(t *testing.T) {
	Convey("Basic SMTP Status", t, func() {
		header, parsed, err := Parse([]byte(`Jun 16 00:07:43 smtpnode07 postfix-10.20.30.40/smtp[3022]: ` +
			`0C31D3D1E6: to=<redacted@aol.com>, relay=mx-aol.mail.gm0.yahoodns.net[11.22.33.44]:25, ` +
			`delay=18910, delays=18900/8.9/0.69/0.03, dsn=4.7.0, status=deferred ` +
			`(host mx-aol.mail.gm0.yahoodns.net[11.22.33.44] said: 421 4.7.0 [TSS04] ` +
			`Messages from 10.20.30.40 temporarily deferred due to user complaints - 4.16.55.1; ` +
			`see https://help.yahoo.com/kb/postmaster/SLN3434.html (in reply to MAIL FROM command))`))
		So(err, ShouldBeNil)
		So(parsed, ShouldNotBeNil)
		p, cast := parsed.(SmtpSentStatus)
		So(cast, ShouldEqual, true)

		So(header.Time.Day, ShouldEqual, 16)
		So(header.Time.Month, ShouldEqual, time.June)
		So(header.Time.Hour, ShouldEqual, 0)
		So(header.Time.Minute, ShouldEqual, 7)
		So(header.Time.Second, ShouldEqual, 43)
		So(header.Host, ShouldEqual, "smtpnode07")
		So(header.Process, ShouldEqual, "postfix")
		So(header.Daemon, ShouldEqual, "smtp")
		So(header.ProcessIP, ShouldEqual, net.ParseIP("10.20.30.40"))
		So(header.PID, ShouldEqual, 3022)

		So(p.Queue, ShouldEqual, "0C31D3D1E6")
		So(p.RecipientLocalPart, ShouldEqual, "redacted")
		So(p.RecipientDomainPart, ShouldEqual, "aol.com")
		So(p.RelayName, ShouldEqual, "mx-aol.mail.gm0.yahoodns.net")

		So(p.RelayIP, ShouldEqual, net.ParseIP("11.22.33.44"))
		So(p.RelayPort, ShouldEqual, 25)
		So(p.Delay, ShouldEqual, 18910)
		So(p.Delays.Smtpd, ShouldEqual, 18900)
		So(p.Delays.Cleanup, ShouldEqual, 8.9)
		So(p.Delays.Qmgr, ShouldEqual, 0.69)
		So(p.Delays.Smtp, ShouldEqual, 0.03)
		So(p.Dsn, ShouldEqual, "4.7.0")
		So(p.Status, ShouldEqual, DeferredStatus)
		So(p.ExtraMessage, ShouldEqual, `(host mx-aol.mail.gm0.yahoodns.net[11.22.33.44] said: 421 4.7.0 [TSS04] `+
			`Messages from 10.20.30.40 temporarily deferred due to user complaints - 4.16.55.1; `+
			`see https://help.yahoo.com/kb/postmaster/SLN3434.html (in reply to MAIL FROM command))`)
	})

	Convey("Basic SMTP Status from different logs", t, func() {
		header, parsed, err := Parse([]byte(`Jul  5 17:24:35 mail postfix/smtp[9635]: D298F2C60812: to=<"user 1234 with space"@icloud.com>,` +
			` relay=mx6.mail.icloud.com[17.178.97.79]:25, delay=428621, delays=428619/0.02/1.9/0, ` +
			`dsn=4.7.0, status=deferred (host mx6.mail.icloud.com[17.178.97.79] ` +
			`refused to talk to me: 550 5.7.0 Blocked - see https://support.proofpoint.com/dnsbl-lookup.cgi?ip=142.93.169.220)`))
		So(parsed, ShouldNotBeNil)
		So(err, ShouldBeNil)
		p, cast := parsed.(SmtpSentStatus)
		So(cast, ShouldEqual, true)

		So(header.Time.Day, ShouldEqual, 5)
		So(header.Time.Month, ShouldEqual, time.July)
		So(header.Time.Hour, ShouldEqual, 17)
		So(header.Time.Minute, ShouldEqual, 24)
		So(header.Time.Second, ShouldEqual, 35)
		So(header.Host, ShouldEqual, "mail")
		So(header.Process, ShouldEqual, "postfix")
		So(header.Daemon, ShouldEqual, "smtp")
		So(header.ProcessIP, ShouldBeNil)

		So(p.Queue, ShouldEqual, "D298F2C60812")
		So(p.RecipientLocalPart, ShouldEqual, "user 1234 with space")
		So(p.RecipientDomainPart, ShouldEqual, "icloud.com")
		So(p.RelayName, ShouldEqual, "mx6.mail.icloud.com")

		So(p.RelayIP, ShouldEqual, net.ParseIP("17.178.97.79"))
		So(p.RelayPort, ShouldEqual, 25)
		So(p.Delay, ShouldEqual, 428621)
		So(p.Delays.Smtpd, ShouldEqual, 428619)
		So(p.Delays.Cleanup, ShouldEqual, 0.02)
		So(p.Delays.Qmgr, ShouldEqual, 1.9)
		So(p.Delays.Smtp, ShouldEqual, 0)
		So(p.Dsn, ShouldEqual, "4.7.0")
		So(p.Status, ShouldEqual, DeferredStatus)
	})

	Convey("A bounced message", t, func() {
		header, parsed, err := Parse([]byte(`Aug  3 04:41:17 mail postfix/smtp[10603]: AE8E32C60819: to=<mail@e.mail.com>, ` +
			`relay=none, delay=0.02, delays=0.01/0/0.01/0, dsn=5.4.4, status=bounced ` +
			`(Host or domain name not found. Name service error for name=e.mail.com type=AAAA: Host not found)`))
		So(parsed, ShouldNotBeNil)
		So(err, ShouldBeNil)
		p, cast := parsed.(SmtpSentStatus)
		So(cast, ShouldEqual, true)

		So(header.Time.Day, ShouldEqual, 3)
		So(header.Time.Month, ShouldEqual, time.August)
		So(header.Time.Hour, ShouldEqual, 4)
		So(header.Time.Minute, ShouldEqual, 41)
		So(header.Time.Second, ShouldEqual, 17)
		So(header.Host, ShouldEqual, "mail")
		So(header.Process, ShouldEqual, "postfix")
		So(header.Daemon, ShouldEqual, "smtp")

		So(p.Queue, ShouldEqual, "AE8E32C60819")
		So(p.RecipientLocalPart, ShouldEqual, "mail")
		So(p.RecipientDomainPart, ShouldEqual, "e.mail.com")
		So(p.OrigRecipientLocalPart, ShouldEqual, "")
		So(p.OrigRecipientDomainPart, ShouldEqual, "")
		So(p.RelayName, ShouldEqual, "")

		So(p.RelayIP, ShouldBeNil)
		So(p.RelayPort, ShouldEqual, 0)
		So(p.Delay, ShouldEqual, 0.02)
		So(p.Delays.Smtpd, ShouldEqual, 0.01)
		So(p.Delays.Cleanup, ShouldEqual, 0)
		So(p.Delays.Qmgr, ShouldEqual, 0.01)
		So(p.Delays.Smtp, ShouldEqual, 0)
		So(p.Dsn, ShouldEqual, "5.4.4")
		So(p.Status, ShouldEqual, BouncedStatus)
	})

	Convey("Log line with optional orig_to", t, func() {
		header, parsed, err := Parse([]byte(`May  5 00:00:00 mail postfix/smtp[17709]: AB5501855DA0: to=<to@mail.com>, ` +
			`orig_to=<orig_to@example.com>, relay=127.0.0.1[127.0.0.1]:10024, delay=0.87, delays=0.68/0.01/0/0.18, ` +
			`dsn=2.0.0, status=sent (250 2.0.0 from MTA(smtp:[127.0.0.1]:10025): 250 2.0.0 Ok: queued as 2F01D1855DB2)`))
		So(parsed, ShouldNotBeNil)
		So(err, ShouldBeNil)
		p, cast := parsed.(SmtpSentStatus)
		So(cast, ShouldBeTrue)

		So(header.Time.Day, ShouldEqual, 5)
		So(header.Time.Month, ShouldEqual, time.May)
		So(header.Time.Hour, ShouldEqual, 0)
		So(header.Time.Minute, ShouldEqual, 0)
		So(header.Time.Second, ShouldEqual, 0)
		So(header.Host, ShouldEqual, "mail")
		So(header.Process, ShouldEqual, "postfix")
		So(header.Daemon, ShouldEqual, "smtp")

		So(p.Queue, ShouldEqual, "AB5501855DA0")
		So(p.RecipientLocalPart, ShouldEqual, "to")
		So(p.RecipientDomainPart, ShouldEqual, "mail.com")
		So(p.OrigRecipientLocalPart, ShouldEqual, "orig_to")
		So(p.OrigRecipientDomainPart, ShouldEqual, "example.com")
		So(p.RelayName, ShouldEqual, "127.0.0.1")
		So(p.Status, ShouldEqual, SentStatus)
		So(p.ExtraMessage, ShouldEqual, `(250 2.0.0 from MTA(smtp:[127.0.0.1]:10025): 250 2.0.0 Ok: queued as 2F01D1855DB2)`)

		e, cast := p.ExtraMessagePayload.(SmtpStatusExtraMessageSentQueued)
		So(cast, ShouldBeTrue)
		So(e.Port, ShouldEqual, 10025)
		So(e.IP, ShouldEqual, net.ParseIP("127.0.0.1"))
		So(e.Queue, ShouldEqual, "2F01D1855DB2")

		// obtained from the beginning of the line, as the values from the end are hard-coded by postfix
		So(e.SmtpCode, ShouldEqual, 250)
		So(e.Dsn, ShouldEqual, "2.0.0")
	})
}

func TestQmgrParsing(t *testing.T) {
	Convey("Qmgr expired message", t, func() {
		header, parsed, err := Parse([]byte(`Sep  3 12:39:14 mailhost postfix-12.34.56.78/qmgr[24086]: ` +
			`B54DA300087: from=<redacted@company.com>, status=expired, returned to sender`))

		So(parsed, ShouldNotBeNil)
		So(err, ShouldBeNil)
		p, cast := parsed.(QmgrReturnedToSender)
		So(cast, ShouldEqual, true)

		So(header.Time.Day, ShouldEqual, 3)
		So(header.Time.Month, ShouldEqual, time.September)
		So(header.Time.Hour, ShouldEqual, 12)
		So(header.Time.Minute, ShouldEqual, 39)
		So(header.Time.Second, ShouldEqual, 14)
		So(header.Host, ShouldEqual, "mailhost")
		So(header.Process, ShouldEqual, "postfix")
		So(header.Daemon, ShouldEqual, "qmgr")
		So(p.SenderLocalPart, ShouldEqual, "redacted")
		So(p.SenderDomainPart, ShouldEqual, "company.com")
		So(p.Queue, ShouldEndWith, "B54DA300087")
	})
}

func TestLmtpParsing(t *testing.T) {
	Convey("Lmtp uses the same struct as SmtpSentStatus", t, func() {
		header, parsed, err := Parse([]byte(`Jan 10 16:15:30 mail postfix/lmtp[11996]: 400643011B47: to=<recipient@example.com>, ` +
			`relay=relay.example.com[/var/run/dovecot/lmtp], delay=0.06, delays=0.02/0.02/0.01/0.01, dsn=2.0.0, status=sent ` +
			`(250 2.0.0 <recipient@example.com> hz3kESIo+1/dLgAAWP5Hkg Saved)`))
		So(parsed, ShouldNotBeNil)
		So(err, ShouldBeNil)
		p, cast := parsed.(SmtpSentStatus)
		So(cast, ShouldBeTrue)

		So(header.Time.Day, ShouldEqual, 10)
		So(header.Time.Month, ShouldEqual, time.January)
		So(header.Time.Hour, ShouldEqual, 16)
		So(header.Time.Minute, ShouldEqual, 15)
		So(header.Time.Second, ShouldEqual, 30)
		So(header.Host, ShouldEqual, "mail")
		So(header.Process, ShouldEqual, "postfix")
		So(header.Daemon, ShouldEqual, "lmtp")

		So(p.Queue, ShouldEqual, "400643011B47")
		So(p.RecipientLocalPart, ShouldEqual, "recipient")
		So(p.RecipientDomainPart, ShouldEqual, "example.com")
		So(p.RelayName, ShouldEqual, "relay.example.com")
		So(p.RelayPath, ShouldEqual, "/var/run/dovecot/lmtp")
		So(p.Status, ShouldEqual, SentStatus)
		So(p.ExtraMessage, ShouldEqual, `(250 2.0.0 <recipient@example.com> hz3kESIo+1/dLgAAWP5Hkg Saved)`)
	})
}

func TestTimeConversion(t *testing.T) {
	Convey("Convert to Unix timestamp on UTC", t, func() {
		t := Time{Day: 25, Month: time.May, Hour: 5, Minute: 12, Second: 22}
		ts := t.Unix(2008, time.UTC)
		So(ts, ShouldEqual, 1211692342)
	})
}

func TestCleanupProcessing(t *testing.T) {
	Convey("Cleanup", t, func() {
		Convey("e-mail like message-id", func() {
			_, payload, err := Parse([]byte("Jun  3 10:40:57 mail postfix/sender-cleanup/cleanup[9709]: 4AA091855DA0: message-id=<ca10035e-2951-bfd5-ec7e-1a5773fce1cd@mail.sender.com>"))
			So(err, ShouldBeNil)
			p, cast := payload.(CleanupMessageAccepted)
			So(cast, ShouldBeTrue)
			So(p.MessageId, ShouldEqual, "ca10035e-2951-bfd5-ec7e-1a5773fce1cd@mail.sender.com")
		})

		Convey("free form like message", func() {
			_, payload, err := Parse([]byte("Jun  3 10:40:57 mail postfix/cleanup[9709]: 4AA091855DA0: message-id=656587JHGJHG"))
			So(err, ShouldBeNil)
			p, cast := payload.(CleanupMessageAccepted)
			So(cast, ShouldBeTrue)
			So(p.MessageId, ShouldEqual, "656587JHGJHG")
		})

	})
}
