// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package transform

import (
	"bufio"
	"github.com/rs/zerolog/log"
	"gitlab.com/lightmeter/controlcenter/logeater/announcer"
	"gitlab.com/lightmeter/controlcenter/pkg/postfix"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"gitlab.com/lightmeter/controlcenter/util/timeutil"
	"io"
	"time"
)

// TODO: unit test this function and try to find edge cases, as there are possibly many!
func ReadFromReader(reader io.Reader, pub postfix.Publisher, build Builder, importAnnouncer announcer.ImportAnnouncer, clock timeutil.Clock) error {
	t, err := build()
	if err != nil {
		return errorutil.Wrap(err)
	}

	// when importing logs from the past (duh!) we expect that the importing progress
	// ends when we reach the moment where the import was triggered,
	// otherwise we will never know when it ends.
	expectedImportEndTime := clock.Now()

	// a totally arbitrary number. It could be anything <= 100
	const numberOfSteps = 100

	var (
		completed           [numberOfSteps]bool
		firstLine           = true
		initialTime         time.Time
		endAlreadyAnnounced = false
	)

	progress := func(t time.Time) uint {
		v := ((t.Unix() - initialTime.Unix()) * numberOfSteps) / (expectedImportEndTime.Unix() - initialTime.Unix())
		return uint(v)
	}

	announceEnd := func(t time.Time) {
		importAnnouncer.AnnounceProgress(announcer.Progress{
			Finished: true,
			Time:     t,
			Progress: 100,
		})

		endAlreadyAnnounced = true
	}

	setupAnnouncerIfNeeded := func(r postfix.Record) {
		if !firstLine {
			return
		}

		firstLine = false
		initialTime = r.Time

		importAnnouncer.AnnounceStart(r.Time)
	}

	announceProgressIfPossible := func(t time.Time) {
		p := progress(t)

		if completed[p] {
			return
		}

		completed[p] = true

		importAnnouncer.AnnounceProgress(announcer.Progress{
			Finished: false,
			Time:     t,
			Progress: int64(p),
		})
	}

	scanner := bufio.NewScanner(reader)

	var r postfix.Record

	for {
		if !scanner.Scan() {
			break
		}

		r, err = t.Transform(scanner.Bytes())
		if err != nil {
			log.Err(err).Msgf("Error reading from reader: %v", reader)
		}

		setupAnnouncerIfNeeded(r)

		pub.Publish(r)

		isPastImportEndTime := r.Time.After(expectedImportEndTime)

		if !isPastImportEndTime {
			announceProgressIfPossible(r.Time)
		}

		if isPastImportEndTime && !endAlreadyAnnounced {
			announceEnd(r.Time)
		}
	}

	if endAlreadyAnnounced {
		return nil
	}

	announceTime := func() time.Time {
		if !r.Time.IsZero() {
			return r.Time
		}

		return expectedImportEndTime
	}()

	announceEnd(announceTime)

	return nil
}
