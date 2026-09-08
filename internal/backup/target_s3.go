package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// s3OpTimeout caps every non-streaming S3 call (stat/list/put-marker/log read-write).
// s3StreamTimeout caps the archive upload, which can legitimately run for many minutes but must
// not hang job.backupMu (and with it every later backup) forever on a black-holed endpoint.
const (
	s3OpTimeout     = 30 * time.Second
	s3StreamTimeout = 2 * time.Hour
)

// S3Config is the connection info for an S3 (or S3-compatible: MinIO, Cloudflare R2, Backblaze B2,
// DigitalOcean Spaces) backup target.
type S3Config struct {
	Endpoint  string // host or host:port, no scheme
	Region    string
	Bucket    string
	Prefix    string // optional key prefix, e.g. "knov/backups/"
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// s3Target stores each backup set as a single <prefix><name>.tar.gz object in a bucket - the
// remote counterpart of localTarget, with the same layout (a sibling <name>.locked marker object
// and a shared log.json events object) so Run/Restore/Rotate work against it unchanged.
type s3Target struct {
	client *minio.Client
	bucket string
	prefix string
}

// NewS3Target returns an S3-backed BackupTarget. Construction does no network I/O - a wrong
// endpoint, missing bucket or bad credentials surface on the first real operation (opening
// /system/backup, or a scheduled backup, which logs the failure).
func NewS3Target(cfg S3Config) (BackupTarget, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 backup target requires KNOV_BACKUP_S3_ENDPOINT and KNOV_BACKUP_S3_BUCKET")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	// tolerate a missing or doubled leading/trailing slash on the configured prefix
	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return &s3Target{client: client, bucket: cfg.Bucket, prefix: prefix}, nil
}

func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s3OpTimeout)
}

func (t *s3Target) archiveKey(name string) string { return t.prefix + name + archiveExt }
func (t *s3Target) lockKey(name string) string    { return t.prefix + name + lockExt }
func (t *s3Target) logKey() string                { return t.prefix + eventLogFile }

func isNotFound(err error) bool {
	return minio.ToErrorResponse(err).StatusCode == http.StatusNotFound
}

func (t *s3Target) exists(key string) (bool, error) {
	ctx, cancel := opCtx()
	defer cancel()
	_, err := t.client.StatObject(ctx, t.bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, err
}

func (t *s3Target) Write(name string, r io.Reader) error {
	exists, err := t.exists(t.archiveKey(name))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("backup set %s already exists", name)
	}
	// s3StreamTimeout, not s3OpTimeout: a large archive can legitimately take minutes to stream
	// up. size -1: streamed multipart upload - the archive's size isn't known up front.
	ctx, cancel := context.WithTimeout(context.Background(), s3StreamTimeout)
	defer cancel()
	_, err = t.client.PutObject(ctx, t.bucket, t.archiveKey(name), r, -1,
		minio.PutObjectOptions{ContentType: "application/gzip", PartSize: 16 * 1024 * 1024})
	return err
}

func (t *s3Target) List() ([]string, error) {
	ctx, cancel := opCtx()
	defer cancel()
	var names []string
	for obj := range t.client.ListObjects(ctx, t.bucket, minio.ListObjectsOptions{Prefix: t.prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		name := strings.TrimPrefix(obj.Key, t.prefix)
		if !strings.HasSuffix(name, archiveExt) || strings.Contains(name, "/") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, archiveExt))
	}
	sort.Strings(names)
	return names, nil
}

func (t *s3Target) Read(name string) (io.ReadCloser, error) {
	// minio GetObject is lazy - it never errors for a missing key, the 404 only surfaces on the
	// first read. Callers (download handler, restore's existence probe, Manifest) rely on a
	// prompt not-found error the way os.Open gives one, so stat the key up front.
	exists, err := t.exists(t.archiveKey(name))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("backup set %s not found", name)
	}
	// no timeout: streamed straight to the caller (download / restore extraction), which controls
	// how long the read lives.
	return t.client.GetObject(context.Background(), t.bucket, t.archiveKey(name), minio.GetObjectOptions{})
}

func (t *s3Target) Delete(name string) error {
	ctx, cancel := opCtx()
	defer cancel()
	if err := t.client.RemoveObject(ctx, t.bucket, t.archiveKey(name), minio.RemoveObjectOptions{}); err != nil {
		return err
	}
	_ = t.client.RemoveObject(ctx, t.bucket, t.lockKey(name), minio.RemoveObjectOptions{}) // best-effort, the archive is already gone either way
	return nil
}

// Lock writes an empty marker object next to name's archive - a separate object rather than
// metadata on the archive, since Rotate needs to check it without fetching the archive.
func (t *s3Target) Lock(name string) error {
	ctx, cancel := opCtx()
	defer cancel()
	_, err := t.client.PutObject(ctx, t.bucket, t.lockKey(name), bytes.NewReader(nil), 0, minio.PutObjectOptions{})
	return err
}

// Unlock removes name's marker object. S3 DELETE is idempotent (no error when the key is already
// gone), matching localTarget.Unlock's "not locked - nothing to do" behaviour.
func (t *s3Target) Unlock(name string) error {
	ctx, cancel := opCtx()
	defer cancel()
	return t.client.RemoveObject(ctx, t.bucket, t.lockKey(name), minio.RemoveObjectOptions{})
}

func (t *s3Target) Locked(name string) (bool, error) {
	return t.exists(t.lockKey(name))
}

func (t *s3Target) LockedNames() (map[string]bool, error) {
	ctx, cancel := opCtx()
	defer cancel()
	locked := make(map[string]bool)
	for obj := range t.client.ListObjects(ctx, t.bucket, minio.ListObjectsOptions{Prefix: t.prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		name := strings.TrimPrefix(obj.Key, t.prefix)
		if strings.HasSuffix(name, lockExt) && !strings.Contains(name, "/") {
			locked[strings.TrimSuffix(name, lockExt)] = true
		}
	}
	return locked, nil
}

// LogEvent appends a new event to the target's history object (a single JSON array, overwritten
// in place) - callers (Run/Restore) are already serialized by job.backupMu, so a plain
// read-modify-write needs no extra locking of its own.
func (t *s3Target) LogEvent(kind EventKind, set string, source EventSource) error {
	events, err := t.Events()
	if err != nil {
		return err
	}
	events = append(events, Event{Kind: kind, Set: set, Time: time.Now(), Source: source})
	data, err := json.Marshal(events)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	_, err = t.client.PutObject(ctx, t.bucket, t.logKey(), bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/json"})
	return err
}

func (t *s3Target) Events() ([]Event, error) {
	// GetObject is lazy and never errors for a missing key, so probe first - a fresh target with
	// no log.json yet just has no events.
	exists, err := t.exists(t.logKey())
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	ctx, cancel := opCtx()
	defer cancel()
	obj, err := t.client.GetObject(ctx, t.bucket, t.logKey(), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}
