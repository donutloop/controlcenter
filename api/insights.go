// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"errors"
	"gitlab.com/lightmeter/controlcenter/httpauth/auth"
	"gitlab.com/lightmeter/controlcenter/httpmiddleware"
	"gitlab.com/lightmeter/controlcenter/insights/core"
	"gitlab.com/lightmeter/controlcenter/pkg/httperror"
	"gitlab.com/lightmeter/controlcenter/recommendation"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"gitlab.com/lightmeter/controlcenter/util/httputil"
	"net/http"
	"strconv"
	"time"
)

type fetchInsightsHandler struct {
	f                          core.Fetcher
	recommendationURLContainer recommendation.URLContainer
}

// @Summary Fetch Insights
// @Produce json
// @Param from query string true "Initial date in the format 1999-12-23"
// @Param to   query string true "Final date in the format 1999-12-23"
// @Param filter query string false "Filter by. Possible values: 'category'" Enums{"category"}
// @Param order query string true "Order by. Possible values: 'creationAsc', 'creationDesc'" Enums{"creationAsc", "creationDesc"}
// @Param entries query int false "Maximum number of insights to fetch"
// @Param category query string false "If filter by category, the category name. Possible values: 'local', 'comparative', 'news', 'intel'" Enums{"local", "comparative", "news", "intel"}
// @Success 200 {object} fetchedInsight
// @Failure 422 {string} string "desc"
// @Router /api/v0/fetchInsights [get]
func (h fetchInsightsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	interval := httpmiddleware.GetIntervalFromContext(r)

	filter := core.BuildFilterByName(r.Form.Get("filter"))
	order := core.BuildOrderByName(r.Form.Get("order"))
	category := core.BuildCategoryByName(r.Form.Get("category"))

	entries, err := func() (int, error) {
		s := r.Form.Get("entries")

		if len(s) == 0 {
			return 0, nil
		}

		return strconv.Atoi(s)
	}()

	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusBadRequest, errorutil.Wrap(err, "Invalid entries query value"))
	}

	if entries < 0 {
		return httperror.NewHTTPStatusCodeError(http.StatusBadRequest, errors.New("Invalid entries query value: negative value"))
	}

	fetchedInsights, err := h.f.FetchInsights(r.Context(), core.FetchOptions{
		Interval:   interval,
		Category:   category,
		FilterBy:   filter,
		OrderBy:    order,
		MaxEntries: entries,
	})

	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(err))
	}

	insights := make(fetchInsightsResult, 0, entries)

	for _, fi := range fetchedInsights {
		i := fetchedInsight{
			ID:          fi.ID(),
			Time:        fi.Time(),
			Rating:      fi.Rating().String(),
			Category:    fi.Category().String(),
			ContentType: fi.ContentType(),
			Content:     fi.Content(),
		}

		if recommendationHelpLinkProvider, ok := fi.Content().(core.RecommendationHelpLinkProvider); ok {
			i.HelpLink = recommendationHelpLinkProvider.HelpLink(h.recommendationURLContainer)
		}

		insights = append(insights, i)
	}

	return httputil.WriteJson(w, insights, http.StatusOK)
}

type fetchedInsight struct {
	ID          int         `json:"id"`
	Time        time.Time   `json:"time"`
	Rating      string      `json:"rating"`
	Category    string      `json:"category"`
	ContentType string      `json:"content_type"`
	Content     interface{} `json:"content"`
	HelpLink    string      `json:"help_link,omitempty"`
}

type fetchInsightsResult []fetchedInsight

func HttpInsights(auth *auth.Authenticator, mux *http.ServeMux, timezone *time.Location, f core.Fetcher) {
	recommendationURLContainer := recommendation.GetDefaultURLContainer()

	mux.Handle("/api/v0/fetchInsights",
		httpmiddleware.WithDefaultStack(auth, httpmiddleware.RequestWithInterval(timezone)).
			WithEndpoint(fetchInsightsHandler{f: f, recommendationURLContainer: recommendationURLContainer}))
}
