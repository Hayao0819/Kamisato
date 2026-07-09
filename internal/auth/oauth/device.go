package oauth

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Hayao0819/Kamisato/internal/buildclient"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

// DeviceRequester starts a device authorization; DevicePoller polls once for the
// token. Both are injectable so the flow is testable without a live server.
type (
	DeviceRequester func(ctx context.Context, serverURL string) (buildclient.DeviceCodeResponse, error)
	DevicePoller    func(ctx context.Context, serverURL, deviceCode string) (buildclient.DeviceTokenResult, error)
)

// deviceOptions is the device-flow analogue of options; kept separate because the
// two flows share little beyond the output writer.
type deviceOptions struct {
	out      io.Writer
	request  DeviceRequester
	poll     DevicePoller
	sleep    func(ctx context.Context, d time.Duration) error
	openURL  BrowserOpener
	noOpen   bool
	timeout  time.Duration
	minSleep time.Duration
}

// DeviceOption configures DeviceLogin.
type DeviceOption func(*deviceOptions)

// WithDeviceOutput sets where the user-facing instructions are written.
func WithDeviceOutput(w io.Writer) DeviceOption { return func(o *deviceOptions) { o.out = w } }

// WithDeviceRequester overrides the device-code request (test seam).
func WithDeviceRequester(fn DeviceRequester) DeviceOption {
	return func(o *deviceOptions) { o.request = fn }
}

// WithDevicePoller overrides the token poll (test seam).
func WithDevicePoller(fn DevicePoller) DeviceOption { return func(o *deviceOptions) { o.poll = fn } }

// WithDeviceSleep overrides the between-polls wait (test seam); the default waits
// the server's interval, honoring ctx cancellation.
func WithDeviceSleep(fn func(ctx context.Context, d time.Duration) error) DeviceOption {
	return func(o *deviceOptions) { o.sleep = fn }
}

// WithDeviceBrowserOpener opens the verification URL automatically when possible.
func WithDeviceBrowserOpener(fn BrowserOpener) DeviceOption {
	return func(o *deviceOptions) { o.openURL = fn }
}

func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// DeviceLogin runs the RFC 8628 device authorization against serverURL: it
// requests a code, shows the user the URL and code to enter in any browser, then
// polls until approved (returning the access token, refresh token, and login),
// denied, or expired.
func DeviceLogin(ctx context.Context, serverURL string, opts ...DeviceOption) (token, refresh, login string, err error) {
	o := deviceOptions{
		out:      os.Stdout,
		request:  buildclient.RequestDeviceCode,
		poll:     buildclient.PollDeviceToken,
		sleep:    ctxSleep,
		minSleep: time.Second,
	}
	for _, apply := range opts {
		apply(&o)
	}

	dc, err := o.request(ctx, serverURL)
	if err != nil {
		return "", "", "", errors.WrapErr(err, "failed to request a device code")
	}

	fmt.Fprintf(o.out, "ブラウザで次の URL を開き、コード %s を入力してください:\n%s\n", dc.UserCode, dc.VerificationURI)
	if dc.VerificationURIComplete != "" {
		fmt.Fprintf(o.out, "（コード入力済みのリンク: %s）\n", dc.VerificationURIComplete)
		if !o.noOpen && o.openURL != nil {
			_ = o.openURL(dc.VerificationURIComplete)
		}
	}

	interval := time.Duration(dc.Interval) * time.Second
	if interval < o.minSleep {
		interval = o.minSleep
	}

	timeout := time.Duration(dc.ExpiresIn) * time.Second
	if o.timeout > 0 {
		timeout = o.timeout
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		res, perr := o.poll(pollCtx, serverURL, dc.DeviceCode)
		if perr != nil {
			return "", "", "", errors.WrapErr(perr, "failed to poll for the device token")
		}
		switch res.Status {
		case "":
			return res.Token, res.Refresh, res.Login, nil
		case "authorization_pending":
			// keep waiting
		case "slow_down":
			// RFC 8628 §3.5: back off by 5s and keep polling.
			interval += 5 * time.Second
		case "access_denied":
			return "", "", "", errors.NewErrf("device login was denied (account not allowed)")
		case "expired_token":
			return "", "", "", errors.NewErrf("device code expired before approval; run login again")
		default:
			return "", "", "", errors.NewErrf("unexpected device authorization status %q", res.Status)
		}
		if serr := o.sleep(pollCtx, interval); serr != nil {
			return "", "", "", errors.WrapErr(serr, "device login timed out or was cancelled")
		}
	}
}
