package httpmiddleware

import (
	"context"
	"errors"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

const DefaultTimeout = time.Second * 30

func WithDefaultTimeout(middleware ...Middleware) Chain {
	return WithTimeout(DefaultTimeout, middleware...)
}

func WithTimeout(timeout time.Duration, middleware ...Middleware) Chain {
	middleware = append(middleware, []Middleware{RequestWithTimeout(timeout)}...)
	return New(middleware...)
}

var (
	// NOTE: this is not an exhaustive parser for Keep-Alive. It's just good enough for our needs
	// More information about Keep-Alive at https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Keep-Alive
	keepAliveRegexp = regexp.MustCompile(`^timeout=(\d+)(, max=.*)?`)

	ErrInvalidKeepAliveHeader = errors.New("Could not parse Keep-Alive Header")
)

func timeoutForRequest(r *http.Request, defaultTimeout time.Duration) (time.Duration, error) {
	keepAlive := r.Header.Get("Keep-Alive")

	if len(keepAlive) == 0 {
		return defaultTimeout, nil
	}

	matches := keepAliveRegexp.FindSubmatch([]byte(keepAlive))

	if len(matches) == 0 {
		return 0, ErrInvalidKeepAliveHeader
	}

	timeout, err := strconv.Atoi(string(matches[1]))

	if err != nil {
		return 0, errorutil.Wrap(err)
	}

	return time.Second * time.Duration(timeout), nil
}

func RequestWithTimeout(defaultTimeout time.Duration) Middleware {
	return func(h CustomHTTPHandler) CustomHTTPHandler {
		return CustomHTTPHandler(func(w http.ResponseWriter, r *http.Request) error {
			now := time.Now()

			timeout, err := timeoutForRequest(r, defaultTimeout)

			if err != nil {
				return NewHTTPStatusCodeError(http.StatusBadRequest, errorutil.Wrap(err, "Error reading Keep-Alive header"))
			}

			ctx, cancel := context.WithTimeout(r.Context(), timeout)

			defer cancel()

			err = h.ServeHTTP(w, r.WithContext(ctx))

			if deadline, ok := ctx.Deadline(); ok && ctx.Err() != nil {
				elapsedTime := deadline.Sub(now)
				return NewHTTPStatusCodeError(http.StatusRequestTimeout, errorutil.Wrap(err, "HTTP request", r.URL.Redacted(), "with timeout of", elapsedTime))
			}

			return nil
		})
	}
}
