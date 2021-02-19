// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

//go:generate go run ./templates/gen_template.go

package email

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	sasl "github.com/emersion/go-sasl"
	smtp "github.com/emersion/go-smtp"
	"gitlab.com/lightmeter/controlcenter/i18n/translator"
	"gitlab.com/lightmeter/controlcenter/meta"
	"gitlab.com/lightmeter/controlcenter/notification/core"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"gitlab.com/lightmeter/controlcenter/util/timeutil"
	"gitlab.com/lightmeter/controlcenter/version"
	"io"
	"net/mail"
	"strings"
	"text/template"
	"time"
)

// TODO: email template (translatable), custom certificate

// this message template is used only by the tests
var messageTemplate = `
	<html>
	<body>
		Description: {{.Description}} <br/>
		Version: {{appVersion}} <br/>
		Translatable: {{translate "Error"}} <br/>
	</body>
	</html>
`

const SettingKey = "messenger_email"

type SecurityType int

const none = "none"

func (t SecurityType) String() string {
	switch t {
	case SecurityTypeNone:
		return none
	case SecurityTypeTLS:
		return "TLS"
	case SecurityTypeSTARTTLS:
		return "STARTTLS"
	default:
		panic("invalid security type")
	}
}

func (t *SecurityType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

var ErrParsingSecurityType = errors.New(`Invalid security type`)

func ParseSecurityType(s string) (SecurityType, error) {
	switch s {
	case none:
		return SecurityTypeNone, nil
	case "STARTTLS":
		return SecurityTypeSTARTTLS, nil
	case "TLS":
		return SecurityTypeTLS, nil
	default:
		return 0, ErrParsingSecurityType
	}
}

const (
	SecurityTypeNone     SecurityType = 0
	SecurityTypeSTARTTLS SecurityType = 1
	SecurityTypeTLS      SecurityType = 2
)

type AuthMethod int

func (m AuthMethod) String() string {
	switch m {
	case AuthMethodNone:
		return none
	case AuthMethodPassword:
		return "password"
	default:
		panic("invalid auth method")
	}
}

func (m *AuthMethod) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

var ErrParsingAuthMethod = errors.New(`Invalid auth method`)

func ParseAuthMethod(s string) (AuthMethod, error) {
	switch s {
	case none:
		return AuthMethodNone, nil
	case "password":
		return AuthMethodPassword, nil
	default:
		return 0, ErrParsingAuthMethod
	}
}

const (
	AuthMethodNone     AuthMethod = 0
	AuthMethodPassword AuthMethod = 1
)

type Settings struct {
	// TODO: use this flag in the policy for the Notifier
	Enabled bool `json:"enabled"`

	Sender     string `json:"sender"`
	Recipients string `json:"recipients"`

	ServerName string `json:"server_name"`
	ServerPort int    `json:"server_port"`

	SecurityType SecurityType `json:"security_type"`
	AuthMethod   AuthMethod   `json:"auth_method"`

	Username string `json:"username"`
	Password string `json:"password"`
}

func addrFromSettings(s Settings) string {
	return fmt.Sprintf("%s:%d", s.ServerName, s.ServerPort)
}

type SettingsFetcher func() (*Settings, error)

type Notifier struct {
	policy          core.Policy
	settingsFetcher SettingsFetcher
	clock           timeutil.Clock
}

func buildTLSConfigFromSettings(settings Settings) *tls.Config {
	// TODO: allow the user to pass custom TLS config, such as custom certificates and CA
	return nil
}

func ValidateSettings(settings Settings) (err error) {
	if err := sendOnClient(settings, func(*smtp.Client) error {
		return nil
	}); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func sendOnClient(settings Settings, actionOnClient func(*smtp.Client) error) (err error) {
	addr := addrFromSettings(settings)

	c, err := func() (*smtp.Client, error) {
		if settings.SecurityType != SecurityTypeTLS {
			c, err := smtp.Dial(addr)
			if err != nil {
				return nil, errorutil.Wrap(err)
			}

			return c, nil
		}

		c, err := smtp.DialTLS(addr, buildTLSConfigFromSettings(settings))

		if err != nil {
			return nil, errorutil.Wrap(err)
		}

		return c, nil
	}()

	if err != nil {
		return errorutil.Wrap(err)
	}

	// this call might fail, and that's fine, as the connection
	// normally closes on Quit()
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok || settings.SecurityType == SecurityTypeSTARTTLS {
		if err := c.StartTLS(buildTLSConfigFromSettings(settings)); err != nil {
			return errorutil.Wrap(err)
		}
	}

	if hasAuth, _ := c.Extension("AUTH"); hasAuth || settings.AuthMethod == AuthMethodPassword {
		// TODO: maybe support OAUTHBEARER (OAuth2) as well?
		auth := sasl.NewPlainClient("", settings.Username, settings.Password)

		if err = c.Auth(auth); err != nil {
			return errorutil.Wrap(err)
		}
	}

	if err := actionOnClient(c); err != nil {
		return errorutil.Wrap(err)
	}

	if err := c.Quit(); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func newWithCustomSettingsFetcherAndClock(policy core.Policy, settingsFetcher SettingsFetcher, clock timeutil.Clock) *Notifier {
	return &Notifier{
		policy:          policy,
		settingsFetcher: settingsFetcher,
		clock:           clock,
	}
}

func NewWithCustomSettingsFetcher(policy core.Policy, settingsFetcher SettingsFetcher) *Notifier {
	return newWithCustomSettingsFetcherAndClock(policy, settingsFetcher, &timeutil.RealClock{})
}

type disabledFromSettingsPolicy struct {
	settingsFetcher SettingsFetcher
}

func (p *disabledFromSettingsPolicy) Reject(core.Notification) (bool, error) {
	s, err := p.settingsFetcher()
	if err != nil {
		return true, errorutil.Wrap(err)
	}

	return !s.Enabled, nil
}

// FIXME: this function is copied from notification/slack!!!
func New(policy core.Policy, reader *meta.Reader) *Notifier {
	fetcher := func() (*Settings, error) {
		s := Settings{}

		if err := reader.RetrieveJson(context.Background(), SettingKey, &s); err != nil {
			return nil, errorutil.Wrap(err)
		}

		return &s, nil
	}

	policies := core.Policies{policy, &disabledFromSettingsPolicy{settingsFetcher: fetcher}}

	return NewWithCustomSettingsFetcher(policies, fetcher)
}

var ErrInvalidEmail = errors.New(`Invalid mail value`)

func buildMessageProperties(translator translator.Translator, n core.Notification, m *Notifier, settings *Settings) (io.Reader, []string, error) {
	template, err := template.New("root").Funcs(template.FuncMap{
		"appVersion": func() string { return version.Version },
		"translate":  translator.Translate,
	}).Parse(messageTemplate)

	if err != nil {
		return nil, nil, errorutil.Wrap(err)
	}

	description, err := core.TranslateNotification(n, translator)
	if err != nil {
		return nil, nil, errorutil.Wrap(err)
	}

	date := m.clock.Now().Format(time.RFC1123Z)

	recipients, err := func() ([]string, error) {
		a, err := mail.ParseAddressList(settings.Recipients)
		if err != nil {
			return []string{}, errorutil.Wrap(err)
		}

		r := []string{}
		for _, v := range a {
			r = append(r, v.Address)
		}

		return r, nil
	}()

	if err != nil {
		return nil, nil, errorutil.Wrap(err)
	}

	// TODO: maybe use a different property for the email subject?
	// Or use a constant?
	subject := n.Content.String()

	headers := map[string]string{
		"To":                        settings.Recipients,
		"From":                      settings.Sender,
		"Date":                      date,
		"Subject":                   subject,
		"User-Agent":                fmt.Sprintf("Lightmeter ControlCenter %v (%v)", version.Version, version.Commit),
		"MIME-Version":              "1.0",
		"Content-Type":              "text/html; charset=UTF-8",
		"Content-Language":          "en-US", // TODO: use language of the translator
		"Content-Transfer-Encoding": "7bit",
	}

	message, err := func() (string, error) {
		var b strings.Builder

		for k, v := range headers {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteString("\r\n")
		}

		b.WriteString("\r\n")

		err := template.Execute(&b, struct {
			Title       string
			Description string
		}{
			Title:       subject,
			Description: description.String(),
		})

		if err != nil {
			return "", errorutil.Wrap(err)
		}

		b.WriteString("\r\n")

		return b.String(), nil
	}()

	if err != nil {
		return nil, nil, errorutil.Wrap(err)
	}

	reader := strings.NewReader(message)

	return reader, recipients, nil
}

// implement Notifier
// TODO: split this function into smaller chunks!!!
func (m *Notifier) Notify(n core.Notification, translator translator.Translator) error {
	reject, err := m.policy.Reject(n)
	if err != nil {
		return errorutil.Wrap(err)
	}

	if reject {
		return nil
	}

	settings, err := m.settingsFetcher()
	if err != nil {
		return errorutil.Wrap(err)
	}

	if settings == nil {
		panic("Settings cannot be nil!")
	}

	onClient := func(c *smtp.Client) error {
		validateEmail := func(s string) error {
			if strings.Contains(s, "\r\n") {
				return ErrInvalidEmail
			}

			return nil
		}

		if err := validateEmail(settings.Sender); err != nil {
			return errorutil.Wrap(err)
		}

		if err := validateEmail(settings.Recipients); err != nil {
			return errorutil.Wrap(err)
		}

		bodyReader, recipients, err := buildMessageProperties(translator, n, m, settings)
		if err != nil {
			return errorutil.Wrap(err)
		}

		if err := c.Mail(settings.Sender, nil); err != nil {
			return errorutil.Wrap(err)
		}

		for _, recipient := range recipients {
			if err := c.Rcpt(recipient); err != nil {
				return errorutil.Wrap(err)
			}
		}

		w, err := c.Data()
		if err != nil {
			return errorutil.Wrap(err)
		}

		_, err = io.Copy(w, bodyReader)
		if err != nil {
			return errorutil.Wrap(err)
		}

		if err := w.Close(); err != nil {
			return errorutil.Wrap(err)
		}

		return nil
	}

	if err := sendOnClient(*settings, onClient); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func (*Notifier) ValidateSettings(s core.Settings) error {
	settings, ok := s.(Settings)

	if !ok {
		return core.ErrInvalidSettings
	}

	if err := ValidateSettings(settings); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func SetSettings(ctx context.Context, writer *meta.AsyncWriter, settings Settings) error {
	if err := writer.StoreJsonSync(ctx, SettingKey, settings); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func GetSettings(ctx context.Context, reader *meta.Reader) (*Settings, error) {
	settings := &Settings{}

	err := reader.RetrieveJson(ctx, SettingKey, settings)
	if err != nil {
		return nil, errorutil.Wrap(err)
	}

	return settings, nil
}