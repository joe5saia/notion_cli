package cmd

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yourorg/notionctl/internal/notion"
)

type syncWatchOptions struct { //nolint:govet // field order favors readability over minimal padding
	initialSince time.Time
	pollInterval time.Duration
	lookback     time.Duration

	dataSourceID  string
	listenAddr    string
	callbackPath  string
	webhookSecret string

	flags uint8
}

func (opts *syncWatchOptions) setDisableWebhook(enabled bool) {
	if enabled {
		opts.flags |= flagDisableWebhook
		return
	}
	opts.flags &^= flagDisableWebhook
}

func (opts *syncWatchOptions) disableWebhookEnabled() bool {
	return opts.flags&flagDisableWebhook != 0
}

func (opts *syncWatchOptions) setSuppressEmpty(enabled bool) {
	if enabled {
		opts.flags |= flagSuppressEmpty
		return
	}
	opts.flags &^= flagSuppressEmpty
}

func (opts *syncWatchOptions) suppressEmptyEnabled() bool {
	return opts.flags&flagSuppressEmpty != 0
}

type changeClient interface {
	QueryDataSource(
		ctx context.Context,
		dataSourceID string,
		req notion.QueryDataSourceRequest,
	) (notion.QueryDataSourceResponse, error)
}

type webhookDelivery struct { //nolint:govet // compact layout not critical relative to clarity
	receivedAt time.Time
	payload    json.RawMessage
	deliveryID string
	eventType  string
}

type watchOutput struct { //nolint:govet // alignment savings negligible for these response payloads
	Window *watchWindow    `json:"window,omitempty"`
	Pages  []notion.Page   `json:"pages,omitempty"`
	Raw    json.RawMessage `json:"raw,omitempty"`

	ReceivedAt time.Time `json:"received_at,omitempty"`
	Kind       string    `json:"kind"`
	EventType  string    `json:"event_type,omitempty"`
	DeliveryID string    `json:"delivery_id,omitempty"`
	Count      int       `json:"count,omitempty"`
}

type watchWindow struct {
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

const (
	defaultWatchListen    = ":8914"
	defaultCallback       = "/webhook"
	defaultPollInterval   = 2 * time.Minute
	defaultLookbackWindow = 10 * time.Minute
	webhookQueueSize      = 16
	webhookMaxBodyBytes   = 1 << 20
	serverReadTimeout     = 5 * time.Second
	serverShutdownTimeout = 3 * time.Second
	defaultPollPageSize   = 100
	flagDisableWebhook    = 1 << 0
	flagSuppressEmpty     = 1 << 1
)

func newSyncWatchCmd(globals *globalOptions) *cobra.Command {
	opts := &syncWatchOptions{
		listenAddr:   defaultWatchListen,
		callbackPath: defaultCallback,
		pollInterval: defaultPollInterval,
		lookback:     defaultLookbackWindow,
	}

	var (
		sinceArg     string
		disableFlag  bool
		suppressFlag bool
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch Notion data source changes via webhooks with polling fallback",
		RunE:  opts.run(globals, &sinceArg, &disableFlag, &suppressFlag),
	}

	cmd.Flags().StringVar(&opts.dataSourceID, "data-source-id", "", "Target Notion data source ID")
	cmd.Flags().StringVar(
		&opts.listenAddr,
		"listen",
		opts.listenAddr,
		"Address to bind the webhook listener (host:port)",
	)
	cmd.Flags().StringVar(
		&opts.callbackPath,
		"callback-path",
		opts.callbackPath,
		"HTTP path for receiving webhook deliveries",
	)
	cmd.Flags().StringVar(
		&opts.webhookSecret,
		"webhook-secret",
		"",
		"Shared secret used to verify Notion webhook signatures",
	)
	cmd.Flags().DurationVar(
		&opts.pollInterval,
		"poll-interval",
		opts.pollInterval,
		"Interval for fallback polling when no webhooks arrive",
	)
	cmd.Flags().DurationVar(
		&opts.lookback,
		"lookback",
		opts.lookback,
		"Initial lookback window when --since is omitted",
	)
	cmd.Flags().StringVar(
		&sinceArg,
		"since",
		"",
		"RFC3339 timestamp to start polling from (overrides --lookback)",
	)
	cmd.Flags().BoolVar(
		&disableFlag,
		"no-webhook",
		false,
		"Disable webhook listener and rely solely on polling",
	)
	cmd.Flags().BoolVar(
		&suppressFlag,
		"suppress-empty",
		false,
		"Suppress poll output when no changes are detected",
	)

	cobra.CheckErr(cmd.MarkFlagRequired("data-source-id"))

	return cmd
}

func (opts *syncWatchOptions) run(
	globals *globalOptions,
	sinceArg *string,
	disableFlag *bool,
	suppressFlag *bool,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if err := opts.prepare(*sinceArg); err != nil {
			return err
		}
		opts.setDisableWebhook(*disableFlag)
		opts.setSuppressEmpty(*suppressFlag)

		client, err := buildClient(globals.profile)
		if err != nil {
			return err
		}

		rt := newWatchRuntime(cmd, opts, client)
		return rt.run()
	}
}

type watchRuntime struct {
	cmd     *cobra.Command
	opts    *syncWatchOptions
	client  changeClient
	encoder *json.Encoder

	deliveries chan webhookDelivery
	errCh      chan error
	ticker     *time.Ticker

	server           *http.Server
	lastPollEnd      time.Time
	lowerExclusiveLB bool
}

func newWatchRuntime(cmd *cobra.Command, opts *syncWatchOptions, client changeClient) *watchRuntime {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)

	return &watchRuntime{
		cmd:        cmd,
		opts:       opts,
		client:     client,
		encoder:    enc,
		deliveries: make(chan webhookDelivery, webhookQueueSize),
		errCh:      make(chan error, 1),
	}
}

func (rt *watchRuntime) run() error {
	ctx, cancel := context.WithCancel(rt.cmd.Context())
	defer cancel()

	if err := rt.startServer(ctx); err != nil {
		return err
	}
	defer rt.stopServer()

	if err := rt.bootstrap(ctx); err != nil {
		return err
	}

	rt.ticker = time.NewTicker(rt.opts.pollInterval)
	defer rt.ticker.Stop()

	return rt.loop(ctx)
}

func (rt *watchRuntime) startServer(ctx context.Context) error {
	if rt.opts.disableWebhookEnabled() {
		return nil
	}
	server, err := rt.opts.startWebhookServer(ctx, rt.cmd, rt.deliveries, rt.errCh)
	if err != nil {
		return err
	}
	rt.server = server
	return nil
}

func (rt *watchRuntime) stopServer() {
	if rt.server == nil {
		return
	}
	rt.opts.shutdownServer(rt.server, rt.cmd.ErrOrStderr())
}

func (rt *watchRuntime) bootstrap(ctx context.Context) error {
	since := rt.opts.initialSince
	if since.IsZero() {
		since = time.Now().UTC().Add(-rt.opts.lookback)
	}
	rt.lastPollEnd = since

	initialUntil := time.Now().UTC()
	if err := rt.opts.emitPoll(
		ctx,
		rt.client,
		rt.encoder,
		rt.lastPollEnd,
		initialUntil,
		false,
	); err != nil {
		return err
	}
	rt.lastPollEnd = initialUntil
	rt.lowerExclusiveLB = true
	return nil
}

func (rt *watchRuntime) loop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-rt.errCh:
			return err
		case delivery := <-rt.deliveries:
			if err := rt.emitWebhook(delivery); err != nil {
				return err
			}
		case <-rt.ticker.C:
			if err := rt.pollNext(ctx); err != nil {
				return err
			}
		}
	}
}

func (rt *watchRuntime) emitWebhook(delivery webhookDelivery) error {
	if err := rt.encoder.Encode(watchOutput{
		Kind:       "webhook",
		EventType:  delivery.eventType,
		DeliveryID: delivery.deliveryID,
		ReceivedAt: delivery.receivedAt,
		Raw:        delivery.payload,
	}); err != nil {
		return fmt.Errorf("write webhook event: %w", err)
	}
	return nil
}

func (rt *watchRuntime) pollNext(ctx context.Context) error {
	until := time.Now().UTC()
	if err := rt.opts.emitPoll(
		ctx,
		rt.client,
		rt.encoder,
		rt.lastPollEnd,
		until,
		rt.lowerExclusiveLB,
	); err != nil {
		return err
	}
	rt.lastPollEnd = until
	rt.lowerExclusiveLB = true
	return nil
}

func (opts *syncWatchOptions) prepare(sinceArg string) error {
	if opts.dataSourceID == "" {
		return errors.New("data-source-id is required")
	}
	if opts.pollInterval <= 0 {
		return errors.New("poll-interval must be greater than zero")
	}
	if sinceArg != "" {
		parsed, err := time.Parse(time.RFC3339, sinceArg)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
		opts.initialSince = parsed.UTC()
	} else if opts.lookback <= 0 {
		return errors.New("lookback must be greater than zero when --since is omitted")
	}
	if opts.callbackPath == "" {
		opts.callbackPath = defaultCallback
	}
	if !strings.HasPrefix(opts.callbackPath, "/") {
		opts.callbackPath = "/" + opts.callbackPath
	}
	return nil
}

func (opts *syncWatchOptions) startWebhookServer(
	ctx context.Context,
	cmd *cobra.Command,
	deliveries chan<- webhookDelivery,
	errCh chan<- error,
) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.Handle(opts.callbackPath, opts.webhookHandler(deliveries, cmd.ErrOrStderr()))

	server := &http.Server{
		Addr:              opts.listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: serverReadTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("webhook server: %w", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("shutdown webhook server: %w", err)
		}
	}()

	if _, err := fmt.Fprintf(
		cmd.ErrOrStderr(),
		"Listening for Notion webhooks on http://%s%s\n",
		server.Addr,
		opts.callbackPath,
	); err != nil {
		return nil, fmt.Errorf("announce webhook listener: %w", err)
	}

	return server, nil
}

func (opts *syncWatchOptions) shutdownServer(server *http.Server, log io.Writer) {
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) && log != nil {
		safeLog(log, "shutdown webhook server: %v", err)
	}
}

func (opts *syncWatchOptions) emitPoll(
	ctx context.Context,
	client changeClient,
	encoder *json.Encoder,
	since,
	until time.Time,
	lowerExclusive bool,
) error {
	if !until.After(since) {
		until = since
	}

	pages, err := fetchChanges(ctx, client, opts.dataSourceID, since, until, lowerExclusive)
	if err != nil {
		return fmt.Errorf("poll changes: %w", err)
	}
	if opts.suppressEmptyEnabled() && len(pages) == 0 {
		return nil
	}

	output := watchOutput{
		Kind: "poll",
		Window: &watchWindow{
			Since: since,
			Until: until,
		},
		Count: len(pages),
		Pages: pages,
	}
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("write poll output: %w", err)
	}
	return nil
}

func (opts *syncWatchOptions) webhookHandler(deliveries chan<- webhookDelivery, log io.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer func() {
			if err := r.Body.Close(); err != nil {
				safeLog(log, "webhook body close error: %v", err)
			}
		}()

		body, err := readWebhookBody(r)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		if !opts.verifySignature(r, body) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		delivery := webhookDelivery{
			payload:    append([]byte(nil), body...),
			deliveryID: r.Header.Get("Notion-Delivery-ID"),
			eventType:  extractEventType(body),
			receivedAt: time.Now().UTC(),
		}

		offerDelivery(deliveries, delivery, log)
		respondWebhookOK(w, log)
	})
}

func (opts *syncWatchOptions) verifySignature(r *http.Request, body []byte) bool {
	if opts.webhookSecret == "" {
		return true
	}

	signature := r.Header.Get("Notion-Signature")
	timestamp := r.Header.Get("Notion-Signature-Timestamp")
	if signature == "" || timestamp == "" {
		return false
	}
	const prefix = "sha256="
	signature = strings.TrimPrefix(signature, prefix)

	mac := hmac.New(sha256.New, []byte(opts.webhookSecret))
	if _, err := mac.Write([]byte(timestamp)); err != nil {
		return false
	}
	if _, err := mac.Write(body); err != nil {
		return false
	}
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func fetchChanges(
	ctx context.Context,
	client changeClient,
	dataSourceID string,
	since,
	until time.Time,
	lowerExclusive bool,
) ([]notion.Page, error) {
	if dataSourceID == "" {
		return nil, errors.New("data source ID cannot be empty")
	}

	lowerKey := "on_or_after"
	if lowerExclusive {
		lowerKey = "after"
	}

	filter := map[string]any{
		"timestamp": "last_edited_time",
		"last_edited_time": map[string]any{
			lowerKey:       since.UTC().Format(time.RFC3339),
			"on_or_before": until.UTC().Format(time.RFC3339),
		},
	}
	sorts := []any{
		map[string]any{
			"timestamp": "last_edited_time",
			"direction": "descending",
		},
	}

	var (
		cursor string
		all    []notion.Page
	)

	for {
		select {
		case <-ctx.Done():
			return all, fmt.Errorf("watch canceled: %w", ctx.Err())
		default:
		}

		req := notion.QueryDataSourceRequest{
			Filter:      filter,
			Sorts:       sorts,
			StartCursor: cursor,
			PageSize:    defaultPollPageSize,
		}

		resp, err := client.QueryDataSource(ctx, dataSourceID, req)
		if err != nil {
			return nil, fmt.Errorf("query data source: %w", err)
		}

		all = append(all, resp.Results...)

		if !resp.HasMore || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	return all, nil
}

func extractEventType(payload []byte) string {
	var outer struct {
		Type  string `json:"type"`
		Event struct {
			Type string `json:"type"`
		} `json:"event"`
	}
	if err := json.Unmarshal(payload, &outer); err != nil {
		return ""
	}
	if outer.Event.Type != "" {
		return outer.Event.Type
	}
	return outer.Type
}

func offerDelivery(deliveries chan<- webhookDelivery, delivery webhookDelivery, log io.Writer) {
	select {
	case deliveries <- delivery:
	default:
		safeLog(log, "dropping webhook delivery: channel full")
	}
}

func respondWebhookOK(w http.ResponseWriter, log io.Writer) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
		safeLog(log, "write webhook ack: %v", err)
	}
}

func readWebhookBody(r *http.Request) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, webhookMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read webhook body: %w", err)
	}
	return data, nil
}

func safeLog(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return
	}
}
