// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package notification

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"gitlab.com/lightmeter/controlcenter/i18n/translator"
	"gitlab.com/lightmeter/controlcenter/meta"
	"gitlab.com/lightmeter/controlcenter/notification/core"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"golang.org/x/text/language"
)

type (
	Notification = core.Notification
	Notifier     = core.Notifier
	Policy       = core.Policy
	Policies     = core.Policies
)

var PassPolicy = core.PassPolicy

func NewWithCustomLanguageFetcher(translators translator.Translators, policy Policy, languageFetcher func() (language.Tag, error), notifiers map[string]Notifier) *Center {
	return &Center{
		translators:   translators,
		notifiers:     notifiers,
		fetchLanguage: languageFetcher,
		policy:        policy,
	}
}

type Settings struct {
	Language string `json:"language"`
}

const SettingKey = "notifications"

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

func New(reader *meta.Reader, translators translator.Translators, policy Policy, notifiers map[string]Notifier) *Center {
	return NewWithCustomLanguageFetcher(translators, policy, func() (language.Tag, error) {
		settings, err := GetSettings(context.Background(), reader)
		if err != nil && errors.Is(err, meta.ErrNoSuchKey) {
			// setting not found
			return language.English, nil
		}

		if err != nil {
			return language.Tag{}, errorutil.Wrap(err)
		}

		tag, err := language.Parse(settings.Language)

		if err != nil {
			return language.Tag{}, errorutil.Wrap(err)
		}

		return tag, nil
	}, notifiers)
}

type Center struct {
	translators   translator.Translators
	notifiers     map[string]Notifier
	fetchLanguage func() (language.Tag, error)
	policy        Policy
}

var ErrInvalidNotifier = errors.New(`Invalid Notifier`)

func (c *Center) Notifier(typ string) (Notifier, error) {
	n, ok := c.notifiers[typ]

	if !ok {
		return nil, ErrInvalidNotifier
	}

	return n, nil
}

func (c *Center) Notify(notification core.Notification) error {
	reject, err := c.policy.Reject(notification)

	if err != nil {
		return errorutil.Wrap(err)
	}

	if reject {
		return nil
	}

	languageTag, err := c.fetchLanguage()
	if err != nil {
		return errorutil.Wrap(err)
	}

	translator := c.translators.Translator(languageTag)

	for k, n := range c.notifiers {
		if err := n.Notify(notification, translator); err != nil {
			log.Warn().Msgf("Error notifying: (%v): %v", k, err)
		}
	}

	return nil
}
